package api

import (
	"encoding/json"
	"net/http"

	"belochka/internal/hub"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates and returns the application HTTP router with all routes mounted.
func NewRouter(h *hub.Hub) http.Handler {
	r := chi.NewRouter()

	r.Get("/api/health", handleHealth)
	r.Get("/api/ws", h.ServeWS)

	return r
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
