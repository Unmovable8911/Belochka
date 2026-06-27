//go:build windows

package assets

import _ "embed"

// Icon is the system tray icon. Windows systray requires ICO format.
//
//go:embed icon.ico
var Icon []byte
