//go:build windows

package main

// hasDesktop always returns true on Windows; the OS always has a shell capable
// of hosting a system tray icon.
func hasDesktop() bool {
	return true
}
