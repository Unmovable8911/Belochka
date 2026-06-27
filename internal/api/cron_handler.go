package api

import (
	"context"
	"net/http"

	"belochka/internal/cron"

	"github.com/go-chi/chi/v5"
)

// CronExecutor runs shell commands on a remote server.
type CronExecutor interface {
	Execute(ctx context.Context, serverID, cmd string) (string, error)
}

// cronHandler handles the cron list endpoint.
type cronHandler struct {
	executor CronExecutor
}

func (h *cronHandler) listCrons(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Use "|| true" to ensure exit 0 even when no crontab exists.
	output, err := h.executor.Execute(r.Context(), id, "crontab -l 2>/dev/null || true")
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to read crontab: "+err.Error())
		return
	}

	result := cron.ParseCrontab(output)
	writeJSON(w, http.StatusOK, result)
}
