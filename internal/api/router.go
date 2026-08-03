package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path/filepath"

	"belochka/internal/auth"
	"belochka/internal/config"
	"belochka/internal/cron"
	"belochka/internal/hub"
	"belochka/internal/process"
	"belochka/internal/ssh"
	"belochka/internal/static"
	"belochka/internal/store"
	"belochka/internal/terminal"

	"github.com/go-chi/chi/v5"
)

// RouterOption configures the router.
type RouterOption func(*routerConfig)

type routerConfig struct {
	staticFS          fs.FS
	langStore         config.ConfigStore
	serverStore       ServerStore
	groupStore        GroupStore
	sshTester         SSHTester
	onServerChange    func()
	onLanguageChange  func(string)
	terminalHandler   *terminal.Handler
	batchRunService   BatchRunService
	cronExecutor      ssh.Executor
	cronRunner        cron.Runner
	processService    *process.Service
	configStore       config.ConfigStore
	keyFileDir        string
	authHandler       *auth.Handler
	authStore         *auth.SessionStore
	reconnecter       Reconnecter
	restartFn         func()
}

// WithStaticFS enables serving embedded frontend assets for non-API routes.
// When not set (development mode), non-API routes return 404.
func WithStaticFS(fsys fs.FS) RouterOption {
	return func(c *routerConfig) {
		c.staticFS = fsys
	}
}

// WithLangStore sets the language store used by the static handler for
// Accept-Language detection and meta tag injection.
func WithLangStore(store config.ConfigStore) RouterOption {
	return func(c *routerConfig) {
		c.langStore = store
	}
}

// WithServerStore sets the store used by server CRUD endpoints.
func WithServerStore(store ServerStore) RouterOption {
	return func(c *routerConfig) {
		c.serverStore = store
	}
}

// WithGroupStore sets the store used by group CRUD endpoints.
func WithGroupStore(store GroupStore) RouterOption {
	return func(c *routerConfig) {
		c.groupStore = store
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

// WithBatchRunService enables the batch command endpoints.
func WithBatchRunService(svc BatchRunService) RouterOption {
	return func(c *routerConfig) {
		c.batchRunService = svc
	}
}

// WithCronExecutor enables the cron list endpoint.
func WithCronExecutor(executor ssh.Executor) RouterOption {
	return func(c *routerConfig) {
		c.cronExecutor = executor
	}
}

// WithCronRunner enables the "run cron now" endpoint.
func WithCronRunner(runner cron.Runner) RouterOption {
	return func(c *routerConfig) {
		c.cronRunner = runner
	}
}

// WithProcessService enables the process management endpoints.
func WithProcessService(svc *process.Service) RouterOption {
	return func(c *routerConfig) {
		c.processService = svc
	}
}

// WithConfigStore enables the GET and PATCH /api/config endpoints.
func WithConfigStore(store config.ConfigStore) RouterOption {
	return func(c *routerConfig) {
		c.configStore = store
	}
}

// WithOnLanguageChange sets a callback invoked when the UI language is changed
// via PATCH /api/config (e.g. to update the system tray menu items).
func WithOnLanguageChange(fn func(string)) RouterOption {
	return func(c *routerConfig) {
		c.onLanguageChange = fn
	}
}

// WithReconnecter enables the POST /api/servers/{id}/reconnect endpoint.
func WithReconnecter(r Reconnecter) RouterOption {
	return func(c *routerConfig) {
		c.reconnecter = r
	}
}

// WithAuth enables authentication on protected routes.
func WithAuth(h *auth.Handler, store *auth.SessionStore) RouterOption {
	return func(c *routerConfig) {
		c.authHandler = h
		c.authStore = store
	}
}

// WithRestartFn sets a callback that triggers a server restart.
// When set, enables the POST /api/restart endpoint.
func WithRestartFn(fn func()) RouterOption {
	return func(c *routerConfig) {
		c.restartFn = fn
	}
}

// WithKeyFileDir sets the directory for storing uploaded SSH key files.
// When set, enables the POST /api/files/key endpoint.
func WithKeyFileDir(dir string) RouterOption {
	return func(c *routerConfig) {
		c.keyFileDir = dir
	}
}

// NewRouter creates and returns the application HTTP router with all routes mounted.
func NewRouter(h *hub.Hub, opts ...RouterOption) http.Handler {
	var cfg routerConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	r := chi.NewRouter()

	// --- Public routes (no auth required) ---
	r.Get("/api/health", handleHealth)

	if cfg.authHandler != nil {
		r.Post("/api/login", cfg.authHandler.HandleLogin)
		r.Post("/api/setup", cfg.authHandler.HandleSetup)
		r.Get("/api/auth/status", cfg.authHandler.HandleAuthStatus)
	}

	// --- Protected routes (auth required when configured) ---
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg.authStore))

		if cfg.authHandler != nil {
			r.Post("/api/logout", cfg.authHandler.HandleLogout)
			r.Post("/api/change-password", cfg.authHandler.HandleChangePassword)
		}

		r.Get("/api/ws", h.ServeWS)

		// Server CRUD and test endpoints
		if cfg.serverStore != nil {
			sh := &serverHandler{store: cfg.serverStore, tester: cfg.sshTester, reconnecter: cfg.reconnecter, onChange: cfg.onServerChange}
			mountServerRoutes(r, sh)
			if cfg.sshTester != nil {
				r.Post("/api/servers/test", sh.testConnection)
			}
			if cfg.reconnecter != nil {
				r.Post("/api/servers/{id}/reconnect", sh.reconnect)
			}
		}

		// Group CRUD endpoints
		if cfg.groupStore != nil {
			gh := &groupHandler{store: cfg.groupStore}
			mountGroupRoutes(r, gh)
		}

		// Key file upload
		if cfg.keyFileDir != "" {
			r.Post("/api/files/key", HandleUploadKey(filepath.Join(cfg.keyFileDir, store.KeysSubDir)))
		}

		// Terminal WebSocket endpoint
		if cfg.terminalHandler != nil {
			r.Get("/api/ws/terminal/{serverID}", cfg.terminalHandler.ServeHTTP)
		}

		// Batch command endpoints
		if cfg.batchRunService != nil {
			bh := &batchRunHandler{service: cfg.batchRunService}
			mountBatchRunRoutes(r, bh)
		}

		// Cron endpoints
		if cfg.cronExecutor != nil {
			ch := &cronHandler{service: cron.NewService(cfg.cronExecutor, cfg.cronRunner)}
			mountCronRoutes(r, ch)
		}

		// Process endpoints
		if cfg.processService != nil {
			ph := &processHandler{service: cfg.processService}
			mountProcessRoutes(r, ph)
		}

		// Config endpoints
		if cfg.configStore != nil {
			ch := &configHandler{store: cfg.configStore, onLanguageChange: cfg.onLanguageChange}
			r.Get("/api/config", ch.getConfig)
			r.Patch("/api/config", ch.patchConfig)
		}

		// Restart endpoint
		if cfg.restartFn != nil {
			rh := &restartHandler{restartFn: cfg.restartFn}
			r.Post("/api/restart", rh.restart)
		}
	})

	// Mount embedded static file serving if available (production mode).
	// Static files and SPA routes are not protected by auth so the login/setup
	// pages can load.
	if handler := static.NewHandler(cfg.staticFS, cfg.langStore); handler != nil {
		r.NotFound(handler.ServeHTTP)
	}

	return r
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func mountServerRoutes(r chi.Router, h *serverHandler) {
	r.Get("/api/servers", h.list)
	r.Post("/api/servers", h.create)
	r.Route("/api/servers/{id}", func(r chi.Router) {
		r.Get("/", h.getByID)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
	})
}

func mountGroupRoutes(r chi.Router, h *groupHandler) {
	r.Get("/api/groups", h.list)
	r.Post("/api/groups", h.create)
	r.Route("/api/groups/{id}", func(r chi.Router) {
		r.Get("/", h.getByID)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
	})
}

func mountCronRoutes(r chi.Router, h *cronHandler) {
	r.Get("/api/servers/{id}/crons", h.listCrons)
	r.Post("/api/servers/{id}/crons", h.createCron)
	r.Put("/api/servers/{id}/crons/{index}", h.updateCron)
	r.Delete("/api/servers/{id}/crons/{index}", h.deleteCron)
	r.Post("/api/servers/{id}/crons/{index}/run", h.runCron)
}

func mountProcessRoutes(r chi.Router, h *processHandler) {
	r.Get("/api/servers/{id}/processes", h.listProcesses)
	r.Post("/api/servers/{id}/processes/{pid}/kill", h.killProcess)
}

func mountBatchRunRoutes(r chi.Router, h *batchRunHandler) {
	r.Post("/api/batch-runs", h.create)
	r.Get("/api/batch-runs/current", h.current)
	r.Post("/api/batch-runs/current/cancel", h.cancel)
	r.Get("/api/ws/batch-runs/{runId}", h.serveWS)
}
