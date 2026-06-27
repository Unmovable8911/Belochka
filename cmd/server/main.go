package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"belochka/internal/app"
	"belochka/internal/config"
	"belochka/internal/logging"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	noTray := flag.Bool("no-tray", false, "disable system tray icon and run as CLI process")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("belochka", version)
		return
	}

	trayMode := hasDesktop() && !*noTray

	logWriter, err := logging.New("./log", !trayMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}

	handler := slog.NewTextHandler(logWriter, nil)
	slog.SetDefault(slog.New(handler))

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

	if trayMode {
		_, port, _ := net.SplitHostPort(a.Addr())
		url := "http://localhost:" + port
		slog.Info("starting in tray mode", "url", url)
		runTray(a, url, ctx, stop) // blocks on main goroutine until quit
		return
	}

	// CLI mode: wait for signal then shut down.
	<-ctx.Done()
	slog.Info("shutting down")

	if err := a.Shutdown(); err != nil {
		slog.Error("graceful shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
