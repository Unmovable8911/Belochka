// Package static serves embedded frontend assets with SPA fallback.
package static

import (
	"io"
	"io/fs"
	"net/http"
)

// NewHandler returns an http.Handler that serves files from the given fs.FS.
// Paths that don't match a file are served index.html (SPA client-side routing).
// Returns nil if fsys is nil (development mode — no embedded assets).
func NewHandler(fsys fs.FS) http.Handler {
	if fsys == nil {
		return nil
	}
	return &spaHandler{fs: http.FileServerFS(fsys), raw: fsys}
}

type spaHandler struct {
	fs  http.Handler
	raw fs.FS
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try to serve the requested file directly.
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Check if the file exists in the embedded filesystem.
	f, err := h.raw.Open(path)
	if err == nil {
		f.Close()
		// File exists — serve it with the standard file server.
		h.fs.ServeHTTP(w, r)
		return
	}

	// File doesn't exist — serve index.html for SPA routing.
	idx, err := h.raw.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	defer idx.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, idx)
}
