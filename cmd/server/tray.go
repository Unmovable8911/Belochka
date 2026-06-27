package main

import (
	"context"
	"log/slog"
	"os/exec"
	"runtime"

	"belochka/assets"
	"belochka/internal/app"

	"fyne.io/systray"
)

// runTray initialises the system tray icon and blocks the calling (main) goroutine
// until the user quits or ctx is cancelled. It must be called from the main goroutine.
func runTray(a *app.Application, url string, ctx context.Context, stop context.CancelFunc) {
	systray.Run(
		func() {
			systray.SetIcon(assets.Icon)
			systray.SetTitle("Belochka")
			systray.SetTooltip("Belochka")
			openItem := systray.AddMenuItem("Open Dashboard", "Open the dashboard in your browser")
			systray.AddSeparator()
			quitItem := systray.AddMenuItem("Quit", "Quit Belochka")

			go func() {
				for {
					select {
					case <-openItem.ClickedCh:
						openBrowser(url)
					case <-quitItem.ClickedCh:
						stop()
						if err := a.Shutdown(); err != nil {
							slog.Error("shutdown error", "error", err)
						}
						systray.Quit()
						return
					case <-ctx.Done():
						if err := a.Shutdown(); err != nil {
							slog.Error("shutdown error", "error", err)
						}
						systray.Quit()
						return
					}
				}
			}()
		},
		func() {}, // onQuit — cleanup already done in the goroutine above
	)
}

// openBrowser opens url in the system default browser without third-party deps.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to open browser", "url", url, "error", err)
	}
}
