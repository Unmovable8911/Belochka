package api

import (
	"log/slog"
	"net/http"
	"time"

	"belochka/internal/httpx"
)

// restartHandler handles POST /api/restart.
type restartHandler struct {
	restartFn func()
}

func (h *restartHandler) restart(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "restarting"})

	// Flush the response before restarting so the client sees the reply.
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	go func() {
		// Small delay to let the HTTP response reach the client.
		time.Sleep(200 * time.Millisecond)
		slog.Info("restart triggered via API")
		h.restartFn()
	}()
}
