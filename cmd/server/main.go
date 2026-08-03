package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"belochka/internal/app"
	"belochka/internal/config"
	"belochka/internal/logging"
)

var version = "dev"

// baseDir returns the directory of the running binary. When the binary is
// run via "go run" (detected by a /tmp/go-build prefix) or os.Executable
// fails, it returns an empty string — signalling callers to fall back to
// the current working directory.
func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	if strings.HasPrefix(dir, "/tmp/go-build") {
		return ""
	}
	return dir
}

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	noTray := flag.Bool("no-tray", false, "disable system tray icon and run as CLI process")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("belochka", version)
		return
	}

	base := baseDir()

	// Load config before anything else; errors go to stderr.
	cfg, err := config.Load(*configPath, base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	resolvedConfigPath := config.ConfigFilePath(*configPath, base)

	trayMode := hasDesktop() && !*noTray

	logPath := cfg.LogPath
	if logPath == "" {
		logPath = config.LogFilePath(base)
	} else {
		logPath = config.ResolvePath(logPath, base)
	}

	retention := time.Duration(cfg.LogRetentionDays) * 24 * time.Hour

	logWriter, err := logging.New(logPath, !trayMode, retention)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}

	handler := slog.NewTextHandler(logWriter, nil)
	slog.SetDefault(slog.New(handler))

	a, err := app.New(cfg, resolvedConfigPath, base)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	a.OnLanguageChange = UpdateTrayLanguage

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
		runTray(a, url, cfg.Language, ctx, stop) // blocks on main goroutine until quit
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

