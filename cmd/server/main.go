package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

type sshTester struct{}

func (sshTester) TestConnection(srv model.Server) (ssh.TestResult, error) {
	return ssh.TestConnection(srv)
}

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	h := hub.New()

	db, err := store.Open(cfg.DataDir, cfg.EncryptionKey)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// SSH connection pool
	pool := ssh.NewPool(db)

	// Metrics collector manager
	collectorMgr := monitor.NewManager(pool, monitor.CollectorOptions{})
	collectorMgr.SetOnFailureThreshold(func(serverID string, failures int) {
		pool.TriggerReconnect(serverID)
	})

	// syncServers ensures the pool and collector manager match the database.
	syncServers := func() {
		servers, err := db.List(context.Background())
		if err != nil {
			slog.Error("failed to list servers for sync", "error", err)
			return
		}

		currentIDs := make(map[string]bool, len(servers))
		for _, s := range servers {
			currentIDs[s.ID] = true
			pool.Add(ctx, s.ID)
			collectorMgr.Add(ctx, s.ID)
		}

		for _, id := range collectorMgr.ServerIDs() {
			if !currentIDs[id] {
				collectorMgr.Remove(id)
				pool.Remove(id)
			}
		}
	}

	// broadcastAll builds and sends a full snapshot to all WebSocket clients.
	broadcastAll := func() {
		servers, err := db.List(context.Background())
		if err != nil {
			slog.Error("failed to list servers for broadcast", "error", err)
			return
		}

		infos := make([]wireServerInfo, len(servers))
		metrics := make(map[string]wireMetrics)

		for i, s := range servers {
			status := pool.Status(s.ID)
			infos[i] = wireServerInfo{
				ID:        s.ID,
				Name:      s.Name,
				Host:      s.Host,
				Status:    string(status.State),
				Attempts:  status.Attempts,
				LastError: status.LastError,
			}

			snap := collectorMgr.Latest(s.ID)
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
		h.SetSnapshot(data)
		h.BroadcastMsg("snapshot", data)
	}

	// Start SSH connections and collectors for existing servers.
	syncServers()
	broadcastAll()

	// Periodic broadcast loop — pushes metrics to all WebSocket clients.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				broadcastAll()
			}
		}
	}()

	onServerChange := func() {
		syncServers()
		broadcastAll()
	}

	var routerOpts []api.RouterOption
	routerOpts = append(routerOpts, api.WithServerStore(db))
	routerOpts = append(routerOpts, api.WithSSHTester(sshTester{}))
	routerOpts = append(routerOpts, api.WithOnServerChange(onServerChange))

	distFS, err := web.DistFS()
	if err != nil {
		slog.Error("failed to load embedded frontend assets", "error", err)
		os.Exit(1)
	}
	routerOpts = append(routerOpts, api.WithStaticFS(distFS))

	router := api.NewRouter(h, routerOpts...)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	hubCtx, hubCancel := context.WithCancel(ctx)

	go h.Run(hubCtx)

	go func() {
		slog.Info("starting server", "addr", srv.Addr, "data_dir", cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	seq := shutdown.NewSequence(shutdownTimeout)

	seq.Add("http", func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	seq.Add("websocket", func(ctx context.Context) error {
		hubCancel()
		return nil
	})

	seq.Add("collectors", func(ctx context.Context) error {
		collectorMgr.StopAll()
		return nil
	})

	seq.Add("ssh", func(ctx context.Context) error {
		pool.CloseAll()
		return nil
	})

	seq.Add("database", func(ctx context.Context) error {
		return db.Close()
	})

	if err := seq.Run(context.Background()); err != nil {
		slog.Error("graceful shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
