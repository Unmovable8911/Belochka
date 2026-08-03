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

// trayLocales holds translated strings for system tray menu items.
// Supported languages: en, zh, fr, ru — mirrors the frontend i18n set.
var trayLocales = map[string]struct {
	openTitle, openTooltip string
	quitTitle, quitTooltip string
}{
	"en": {"Open Dashboard", "Open the dashboard in your browser", "Quit", "Quit Belochka"},
	"zh": {"打开仪表板", "在浏览器中打开仪表板", "退出", "退出 Belochka"},
	"fr": {"Ouvrir le tableau de bord", "Ouvrir le tableau de bord dans le navigateur", "Quitter", "Quitter Belochka"},
	"ru": {"Открыть панель", "Открыть панель в браузере", "Выход", "Выйти из Belochka"},
}

// trayMenuItems holds references to localised menu items so they can be
// updated at runtime when the user changes language via the web UI.
var trayMenuItems struct {
	open *systray.MenuItem
	quit *systray.MenuItem
}

// UpdateTrayLanguage updates the title and tooltip of localised tray menu
// items to match the given language. Safe to call from any goroutine, and
// safe to call before menu items have been created (no-op).
func UpdateTrayLanguage(lang string) {
	loc, ok := trayLocales[lang]
	if !ok {
		loc = trayLocales["en"]
	}
	if trayMenuItems.open != nil {
		trayMenuItems.open.SetTitle(loc.openTitle)
		trayMenuItems.open.SetTooltip(loc.openTooltip)
	}
	if trayMenuItems.quit != nil {
		trayMenuItems.quit.SetTitle(loc.quitTitle)
		trayMenuItems.quit.SetTooltip(loc.quitTooltip)
	}
}

// runTray initialises the system tray icon and blocks the calling (main) goroutine
// until the user quits or ctx is cancelled. It must be called from the main goroutine.
func runTray(a *app.Application, url string, lang string, ctx context.Context, stop context.CancelFunc) {
	loc, ok := trayLocales[lang]
	if !ok {
		loc = trayLocales["en"]
	}

	systray.Run(
		func() {
			systray.SetIcon(assets.Icon)
			systray.SetTitle("Belochka")
			systray.SetTooltip("Belochka")
			openItem := systray.AddMenuItem(loc.openTitle, loc.openTooltip)
			systray.AddSeparator()
			quitItem := systray.AddMenuItem(loc.quitTitle, loc.quitTooltip)
			trayMenuItems.open = openItem
			trayMenuItems.quit = quitItem

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
		return
	}
	// Reap the child so it does not linger as a zombie for the lifetime of the
	// long-running tray process across repeated "Open Dashboard" clicks.
	go func() { _ = cmd.Wait() }()
}
