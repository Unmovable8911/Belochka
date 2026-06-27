//go:build !windows

package main

import "os"

// hasDesktop reports whether the current environment has a graphical desktop
// capable of hosting a system tray icon.
// On Linux/macOS: true if DISPLAY or WAYLAND_DISPLAY is set.
// On Windows: always true (handled by build-tag file).
func hasDesktop() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}
