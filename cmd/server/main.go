package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"belochka/internal/app"
	"belochka/internal/config"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	configPath := flag.String("config", "", "path to configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("belochka", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	a, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := a.Start(ctx); err != nil {
		slog.Error("failed to start application", "error", err)
		os.Exit(1)
	}

	<-ctx.Done()
	slog.Info("shutting down")

	if err := a.Shutdown(); err != nil {
		slog.Error("graceful shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
