// Package web provides the embedded frontend assets built by Vite.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns a filesystem rooted at the dist/ directory,
// containing the production-built frontend assets.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
