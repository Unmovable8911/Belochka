package api

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"belochka/internal/hub"
	"belochka/internal/static"

	"github.com/go-chi/chi/v5"
)

// RouterOption configures the router.
type RouterOption func(*routerConfig)

type routerConfig struct {
	staticFS fs.FS
}

// WithStaticFS enables serving embedded frontend assets for non-API routes.
// When not set (development mode), non-API routes return 404.
func WithStaticFS(fsys fs.FS) RouterOption {
	return func(c *routerConfig) {
		c.staticFS = fsys
	}
}

// NewRouter creates and returns the application HTTP router with all routes mounted.
func NewRouter(h *hub.Hub, opts ...RouterOption) http.Handler {
	var cfg routerConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	r := chi.NewRouter()

	r.Get("/api/health", handleHealth)
	r.Get("/api/ws", h.ServeWS)

	// Mount embedded static file serving if available (production mode).
	if handler := static.NewHandler(cfg.staticFS); handler != nil {
		r.NotFound(handler.ServeHTTP)
	}

	return r
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
