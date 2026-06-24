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

	broadcastServers := func() {
		servers, err := db.List(context.Background())
		if err != nil {
			slog.Error("failed to list servers for broadcast", "error", err)
			return
		}
		type serverInfo struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Host   string `json:"host"`
			Status string `json:"status"`
		}
		infos := make([]serverInfo, len(servers))
		for i, s := range servers {
			infos[i] = serverInfo{ID: s.ID, Name: s.Name, Host: s.Host, Status: "disconnected"}
		}
		snapData, err := json.Marshal(map[string]interface{}{
			"servers": infos,
			"metrics": map[string]interface{}{},
		})
		if err != nil {
			slog.Error("failed to marshal snapshot", "error", err)
			return
		}
		h.SetSnapshot(snapData)
		h.BroadcastMsg("snapshot", snapData)
	}

	broadcastServers()

	var routerOpts []api.RouterOption
	routerOpts = append(routerOpts, api.WithServerStore(db))
	routerOpts = append(routerOpts, api.WithSSHTester(sshTester{}))
	routerOpts = append(routerOpts, api.WithOnServerChange(broadcastServers))

	// Embed production frontend assets.
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

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

	// Step 1: Stop accepting new HTTP connections.
	seq.Add("http", func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	// Step 2: Close WebSocket connections (sends close frames).
	seq.Add("websocket", func(ctx context.Context) error {
		hubCancel()
		return nil
	})

	// Step 3: Stop all collector goroutines.
	// (monitor.Manager is wired here when available)

	// Step 4: Close all SSH connections.
	// (SSH pool is closed here when available)

	// Step 5: Close SQLite database (WAL checkpoint).
	seq.Add("database", func(ctx context.Context) error {
		return db.Close()
	})

	if err := seq.Run(context.Background()); err != nil {
		slog.Error("graceful shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
