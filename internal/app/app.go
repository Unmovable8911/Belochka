package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"belochka/internal/api"
	"belochka/internal/auth"
	"belochka/internal/batchrun"
	"belochka/internal/clock"
	"belochka/internal/config"
	"belochka/internal/hub"
	"belochka/internal/monitor"
	"belochka/internal/process"
	"belochka/internal/pty"
	"belochka/internal/shutdown"
	"belochka/internal/ssh"
	"belochka/internal/store"
	"belochka/internal/terminal"
	"belochka/web"
)

const shutdownTimeout = 10 * time.Second
const broadcastInterval = 2 * time.Second

// Application is the top-level container for all server components.
// It wires together the hub, store, SSH pool, collector manager,
// and HTTP server, providing Start/Shutdown lifecycle management.
// Business logic (server sync, broadcast assembly) is delegated to
// focused components behind narrow interfaces so they can be tested
// independently of the full DI graph.
type Application struct {
	cfg              config.Config
	configStore      *config.Store
	sessionStore     *auth.SessionStore
	hub              *hub.Hub        // lifecycle: Run, ServeWS
	store            *store.SQLiteStore
	pool             *ssh.Pool
	collectorMgr     *monitor.Manager
	synchronizer     *ServerSynchronizer
	broadcastSvc     *BroadcastService
	terminalHandler  *terminal.Handler
	batchRunSvc      *batchrun.Service
	httpServer       *http.Server
	hubCancel        context.CancelFunc
	addr             string
	clock            clock.Clock
	dataDir          string
	OnLanguageChange func(string)
}

// New creates a new Application from the given configuration.
// configPath is the path to the config file used for PATCH /api/config persistence;
// pass an empty string to disable file persistence (config remains in-memory only).
// baseDir is the directory of the running binary, used to resolve relative paths.
// It opens the database and initialises the hub, SSH pool, and
// collector manager. Returns an error if the database cannot be opened.
func New(cfg config.Config, configPath string, baseDir string) (*Application, error) {
	h := hub.New()

	dataDir := config.ResolvePath(cfg.DataDir, baseDir)
	db, err := store.Open(dataDir, os.Getenv("BELOCHKA_ENCRYPTION_KEY"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	pool := ssh.NewPool(db, clock.Real{})

	collectorMgr := monitor.NewManager(pool, monitor.CollectorOptions{}, clock.Real{})
	collectorMgr.SetOnFailureThreshold(func(serverID string, failures int) {
		pool.TriggerReconnect(serverID)
	})

	ptyOpener := pty.NewOpener(pool.OpenSession)

	termHandler := terminal.NewHandler(ptyOpener)

	configStore := config.NewStore(cfg, configPath, baseDir)
	// Write default config to disk on first run when no config file exists
	// and we have a path to write to.
	if configPath != "" {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if err := configStore.Update(func(c *config.Config) { *c = cfg }); err != nil {
				return nil, fmt.Errorf("write initial config: %w", err)
			}
		}
	}

	sessionStore := auth.NewSessionStore(configStore, clock.Real{})

	synchronizer := NewServerSynchronizer(db, pool, collectorMgr)
	broadcastSvc := NewBroadcastService(db, pool, collectorMgr, h)
	batchRunSvc := batchrun.New(db, pool, ptyOpener)

	return &Application{
		cfg:             cfg,
		configStore:     configStore,
		sessionStore:    sessionStore,
		hub:             h,
		store:           db,
		pool:            pool,
		collectorMgr:    collectorMgr,
		synchronizer:    synchronizer,
		broadcastSvc:    broadcastSvc,
		terminalHandler: termHandler,
		batchRunSvc:     batchRunSvc,
		clock:           clock.Real{},
		dataDir:         dataDir,
	}, nil
}

// Start binds the HTTP listener, starts the hub goroutine, the
// broadcast ticker, and the HTTP server. It returns once the port
// is bound (no time.Sleep needed in tests). Returns an error if
// the listener cannot be created.
func (a *Application) Start(ctx context.Context) error {
	a.synchronizer.SyncAndLog(ctx)
	a.broadcastSvc.Broadcast(ctx)

	// Orphan cleanup runs async to avoid blocking startup.
	go func() { _ = a.store.CleanupOrphanKeys(a.dataDir) }()

	onServerChange := func() {
		a.synchronizer.SyncAndLog(ctx)
		a.broadcastSvc.Broadcast(ctx)
		go func() { _ = a.store.CleanupOrphanKeys(a.dataDir) }()
	}

	var routerOpts []api.RouterOption
	routerOpts = append(routerOpts, api.WithServerStore(a.store))
	routerOpts = append(routerOpts, api.WithGroupStore(a.store))
	routerOpts = append(routerOpts, api.WithSSHTester(ssh.TestConnection))
	routerOpts = append(routerOpts, api.WithOnServerChange(onServerChange))
	routerOpts = append(routerOpts, api.WithTerminalHandler(a.terminalHandler))
	routerOpts = append(routerOpts, api.WithCronExecutor(a.pool))
	routerOpts = append(routerOpts, api.WithCronRunner(a.pool))
	routerOpts = append(routerOpts, api.WithProcessService(process.NewService(a.pool)))
	routerOpts = append(routerOpts, api.WithBatchRunService(a.batchRunSvc))
	routerOpts = append(routerOpts, api.WithConfigStore(a.configStore))
	routerOpts = append(routerOpts, api.WithRestartFn(a.Restart))
	routerOpts = append(routerOpts, api.WithLangStore(a.configStore))
	routerOpts = append(routerOpts, api.WithOnLanguageChange(a.OnLanguageChange))
	routerOpts = append(routerOpts, api.WithKeyFileDir(a.dataDir))
	routerOpts = append(routerOpts, api.WithReconnecter(a.pool))
	routerOpts = append(routerOpts, api.WithAuth(auth.NewHandler(a.sessionStore), a.sessionStore))

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

	// Periodically purge expired sessions and rate-limit state.
	go func() {
		ticker := a.clock.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C():
				a.sessionStore.CleanupExpired()
			}
		}
	}()

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

	seq.Add("terminal", func(ctx context.Context) error {
		a.terminalHandler.CloseAll()
		return nil
	})

	seq.Add("batch", func(ctx context.Context) error {
		// Terminate any in-flight Batch Run; its results are persisted as
		// cancelled by the background executions.
		a.batchRunSvc.Cancel()
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

// Restart gracefully shuts down the application and replaces the current
// process with a new instance of the same binary. It never returns on success.
func (a *Application) Restart() {
	if err := a.Shutdown(); err != nil {
		slog.Error("shutdown before restart had errors", "error", err)
	}

	exe, err := os.Executable()
	if err != nil {
		slog.Error("cannot find executable for restart", "error", err)
		os.Exit(1)
	}

	// syscall.Exec replaces the current process image, preserving the PID.
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		slog.Error("restart failed", "error", err)
		os.Exit(1)
	}
}

// runBroadcastLoop periodically broadcasts metrics to all WebSocket
// clients until ctx is cancelled.
func (a *Application) runBroadcastLoop(ctx context.Context) {
	ticker := a.clock.NewTicker(broadcastInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			a.broadcastSvc.Broadcast(ctx)
		}
	}
}
