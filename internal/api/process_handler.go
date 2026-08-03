package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"belochka/internal/httpx"
	"belochka/internal/model"
	"belochka/internal/process"

	"github.com/go-chi/chi/v5"
)

// processHandler handles process endpoints.
type processHandler struct {
	service *process.Service
}

func (h *processHandler) listProcesses(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	processes, err := h.service.List(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "ssh_error", "Failed to list processes: "+err.Error())
		return
	}

	if processes == nil {
		processes = []model.Process{}
	}

	httpx.WriteJSON(w, http.StatusOK, processes)
}

type killProcessRequest struct {
	Signal string `json:"signal"`
}

type killProcessResponse struct {
	PID     int    `json:"pid"`
	Signal  string `json:"signal"`
	Success bool   `json:"success"`
}

func (h *processHandler) killProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pid, err := strconv.Atoi(chi.URLParam(r, "pid"))
	if err != nil || pid <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_pid", "pid must be a positive integer")
		return
	}

	var req killProcessRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
	}

	if req.Signal == "" {
		req.Signal = "SIGTERM"
	}

	if err := h.service.Kill(r.Context(), id, pid, req.Signal); err != nil {
		if errors.Is(err, process.ErrProtectedProcess) {
			httpx.WriteError(w, http.StatusForbidden, "protected_process", fmt.Sprintf("process %d is protected", pid))
			return
		}
		if errors.Is(err, process.ErrProcessNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", fmt.Sprintf("process %d not found", pid))
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "ssh_error", "Failed to kill process: "+err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, killProcessResponse{
		PID:     pid,
		Signal:  req.Signal,
		Success: true,
	})
}
