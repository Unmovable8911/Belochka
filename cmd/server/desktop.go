//go:build !windows && !darwin

package main

import "os"

// hasDesktop reports whether the current environment has a graphical desktop
// capable of hosting a system tray icon.
// On Linux/BSD: true if DISPLAY or WAYLAND_DISPLAY is set.
// macOS and Windows are handled by their own build-tag files (always true).
func hasDesktop() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}
