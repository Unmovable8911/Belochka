//go:build !windows

package assets

import _ "embed"

// Icon is the system tray icon. PNG is accepted by systray on Linux and macOS.
//
//go:embed icon.png
var Icon []byte
