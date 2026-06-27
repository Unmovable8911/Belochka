//go:build darwin

package main

// hasDesktop always returns true on macOS. Native macOS has no DISPLAY
// environment variable, but the Aqua window server is always present to host a
// menu bar / system tray icon.
func hasDesktop() bool {
	return true
}
