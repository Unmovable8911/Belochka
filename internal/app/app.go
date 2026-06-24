package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"belochka/internal/api"
	"belochka/internal/config"
	"belochka/internal/hub"
	"belochka/internal/model"
	"belochka/internal/monitor"
	"belochka/internal/shutdown"
	"belochka/internal/ssh"
	"belochka/internal/store"
	"belochka/web"
)

const shutdownTimeout = 10 * time.Second

type sshTester struct{}

func (sshTester) TestConnection(srv model.Server) (ssh.TestResult, error) {
	return ssh.TestConnection(srv)
}

// Application is the top-level container for all server components.
// It wires together the hub, store, SSH pool, collector manager,
// and HTTP server, providing Start/Shutdown lifecycle management.
type Application struct {
	cfg          config.Config
	hub          *hub.Hub
	store        *store.SQLiteStore
	pool         *ssh.Pool
	collectorMgr *monitor.Manager
	httpServer   *http.Server
	hubCancel    context.CancelFunc
	addr         string
}

// New creates a new Application from the given configuration.
// It opens the database and initialises the hub, SSH pool, and
// collector manager. Returns an error if the database cannot be opened.
func New(cfg config.Config) (*Application, error) {
	h := hub.New()

	db, err := store.Open(cfg.DataDir, cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	pool := ssh.NewPool(db)

	collectorMgr := monitor.NewManager(pool, monitor.CollectorOptions{})
	collectorMgr.SetOnFailureThreshold(func(serverID string, failures int) {
		pool.TriggerReconnect(serverID)
	})

	return &Application{
		cfg:          cfg,
		hub:          h,
		store:        db,
		pool:         pool,
		collectorMgr: collectorMgr,
	}, nil
}

// Start binds the HTTP listener, starts the hub goroutine, the
// broadcast ticker, and the HTTP server. It returns once the port
// is bound (no time.Sleep needed in tests). Returns an error if
// the listener cannot be created.
func (a *Application) Start(ctx context.Context) error {
	a.syncServers(ctx)
	a.broadcastAll(ctx)

	onServerChange := func() {
		a.syncServers(ctx)
		a.broadcastAll(ctx)
	}

	var routerOpts []api.RouterOption
	routerOpts = append(routerOpts, api.WithServerStore(a.store))
	routerOpts = append(routerOpts, api.WithSSHTester(sshTester{}))
	routerOpts = append(routerOpts, api.WithOnServerChange(onServerChange))

	distFS, err := web.DistFS()
	if err != nil {
		return fmt.Errorf("load frontend assets: %w", err)
	}
	routerOpts = append(routerOpts, api.WithStaticFS(distFS))

	router := api.NewRouter(a.hub, routerOpts...)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", a.cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	a.addr = ln.Addr().String()

	a.httpServer = &http.Server{Handler: router}

	hubCtx, hubCancel := context.WithCancel(ctx)
	a.hubCancel = hubCancel
	go a.hub.Run(hubCtx)

	go a.runBroadcastLoop(ctx)

	go func() {
		slog.Info("starting server", "addr", a.addr, "data_dir", a.cfg.DataDir)
		if err := a.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
		}
	}()

	return nil
}

// Shutdown performs ordered graceful shutdown: HTTP server, hub,
// collectors, SSH pool, and database.
func (a *Application) Shutdown() error {
	seq := shutdown.NewSequence(shutdownTimeout)

	seq.Add("http", func(ctx context.Context) error {
		return a.httpServer.Shutdown(ctx)
	})

	seq.Add("websocket", func(ctx context.Context) error {
		a.hubCancel()
		return nil
	})

	seq.Add("collectors", func(ctx context.Context) error {
		a.collectorMgr.StopAll()
		return nil
	})

	seq.Add("ssh", func(ctx context.Context) error {
		a.pool.CloseAll()
		return nil
	})

	seq.Add("database", func(ctx context.Context) error {
		return a.store.Close()
	})

	return seq.Run(context.Background())
}

// Addr returns the bound network address (host:port). Useful when
// Port=0 is used to get an OS-assigned port.
func (a *Application) Addr() string {
	return a.addr
}

// syncServers ensures the SSH pool and collector manager match the
// current set of servers in the database.
func (a *Application) syncServers(ctx context.Context) {
	servers, err := a.store.List(ctx)
	if err != nil {
		slog.Error("failed to list servers for sync", "error", err)
		return
	}

	currentIDs := make(map[string]bool, len(servers))
	for _, s := range servers {
		currentIDs[s.ID] = true
		a.pool.Add(ctx, s.ID)
		a.collectorMgr.Add(ctx, s.ID)
	}

	for _, id := range a.collectorMgr.ServerIDs() {
		if !currentIDs[id] {
			a.collectorMgr.Remove(id)
			a.pool.Remove(id)
		}
	}
}

// broadcastAll builds a full snapshot from the current server list
// and broadcasts it to all WebSocket clients.
func (a *Application) broadcastAll(ctx context.Context) {
	servers, err := a.store.List(ctx)
	if err != nil {
		slog.Error("failed to list servers for broadcast", "error", err)
		return
	}

	infos := make([]wireServerInfo, len(servers))
	metrics := make(map[string]wireMetrics)

	for i, s := range servers {
		status := a.pool.Status(s.ID)
		infos[i] = wireServerInfo{
			ID:        s.ID,
			Name:      s.Name,
			Host:      s.Host,
			Status:    string(status.State),
			Attempts:  status.Attempts,
			LastError: status.LastError,
		}

		snap := a.collectorMgr.Latest(s.ID)
		if snap != nil {
			metrics[s.ID] = snapshotToWire(*snap)
		}
	}

	payload := wireSnapshot{
		Servers: infos,
		Metrics: metrics,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal snapshot", "error", err)
		return
	}
	a.hub.SetSnapshot(data)
	a.hub.BroadcastMsg("snapshot", data)
}

// runBroadcastLoop periodically broadcasts metrics to all WebSocket
// clients until ctx is cancelled.
func (a *Application) runBroadcastLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.broadcastAll(ctx)
		}
	}
}
