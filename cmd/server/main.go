package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"belochka/internal/api"
	"belochka/internal/hub"
	"belochka/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	h := hub.New()

	var routerOpts []api.RouterOption

	// Embed production frontend assets.
	distFS, err := web.DistFS()
	if err != nil {
		slog.Error("failed to load embedded frontend assets", "error", err)
		os.Exit(1)
	}
	routerOpts = append(routerOpts, api.WithStaticFS(distFS))

	router := api.NewRouter(h, routerOpts...)

	srv := &http.Server{
		Addr:    ":53136",
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go h.Run(ctx)

	go func() {
		slog.Info("starting server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
