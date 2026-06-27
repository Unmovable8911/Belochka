package api

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"belochka/internal/hub"
	"belochka/internal/static"
	"belochka/internal/terminal"

	"github.com/go-chi/chi/v5"
)

// RouterOption configures the router.
type RouterOption func(*routerConfig)

type routerConfig struct {
	staticFS        fs.FS
	serverStore     ServerStore
	sshTester       SSHTester
	onServerChange  func()
	terminalHandler *terminal.Handler
	cronExecutor    CronExecutor
}

// WithStaticFS enables serving embedded frontend assets for non-API routes.
// When not set (development mode), non-API routes return 404.
func WithStaticFS(fsys fs.FS) RouterOption {
	return func(c *routerConfig) {
		c.staticFS = fsys
	}
}

// WithServerStore sets the store used by server CRUD endpoints.
func WithServerStore(store ServerStore) RouterOption {
	return func(c *routerConfig) {
		c.serverStore = store
	}
}

// WithSSHTester sets the SSH tester used by the connection test endpoint.
func WithSSHTester(tester SSHTester) RouterOption {
	return func(c *routerConfig) {
		c.sshTester = tester
	}
}

// WithOnServerChange sets a callback invoked after server create/update/delete.
func WithOnServerChange(fn func()) RouterOption {
	return func(c *routerConfig) {
		c.onServerChange = fn
	}
}

// WithTerminalHandler enables the terminal WebSocket endpoint.
func WithTerminalHandler(h *terminal.Handler) RouterOption {
	return func(c *routerConfig) {
		c.terminalHandler = h
	}
}

// WithCronExecutor enables the cron list endpoint.
func WithCronExecutor(executor CronExecutor) RouterOption {
	return func(c *routerConfig) {
		c.cronExecutor = executor
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

	// Server CRUD and test endpoints
	if cfg.serverStore != nil {
		sh := &serverHandler{store: cfg.serverStore, tester: cfg.sshTester, onChange: cfg.onServerChange}
		r.Post("/api/servers", sh.create)
		r.Get("/api/servers", sh.list)
		r.Get("/api/servers/{id}", sh.getByID)
		r.Put("/api/servers/{id}", sh.update)
		r.Delete("/api/servers/{id}", sh.delete)
		if cfg.sshTester != nil {
			r.Post("/api/servers/test", sh.testConnection)
		}
	}

	// Terminal WebSocket endpoint
	if cfg.terminalHandler != nil {
		r.Get("/api/ws/terminal/{serverID}", cfg.terminalHandler.ServeHTTP)
	}

	// Cron list endpoint
	if cfg.cronExecutor != nil {
		ch := &cronHandler{executor: cfg.cronExecutor}
		r.Get("/api/servers/{id}/crons", ch.listCrons)
	}

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
