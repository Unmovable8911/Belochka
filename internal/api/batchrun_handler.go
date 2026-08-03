package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"belochka/internal/batchrun"
	"belochka/internal/httpx"
	"belochka/internal/model"
	"belochka/internal/wsutil"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// BatchRunService is the subset of batchrun.Service used by the batch command
// endpoints.
type BatchRunService interface {
	Dispatch(ctx context.Context, script string, serverIDs []string) (model.BatchRun, []model.RunResult, error)
	Current(ctx context.Context) (model.BatchRun, []model.RunResult, error)
	Cancel()
	Input(runID, serverID string, data []byte) error
	Subscribe(runID string) (<-chan batchrun.Event, func(), error)
}

// batchRunHandler handles the batch command endpoints.
type batchRunHandler struct {
	service BatchRunService
}

var batchWSUpgrader = websocket.Upgrader{
	CheckOrigin: wsutil.CheckOrigin,
}

// wsInput is a client→server frame carrying interactive stdin data.
type wsInput struct {
	Type     string `json:"type"`
	ServerID string `json:"server_id"`
	Data     string `json:"data"`
}

type batchRunRequest struct {
	Script    string   `json:"script"`
	ServerIDs []string `json:"server_ids"`
}

type batchRunResponse struct {
	Run     batchRunInfo    `json:"run"`
	Results []runResultInfo `json:"results"`
}

type batchRunInfo struct {
	ID        string `json:"id"`
	Script    string `json:"script"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type runResultInfo struct {
	ServerID   string  `json:"server_id"`
	Status     string  `json:"status"`
	ExitCode   *int    `json:"exit_code,omitempty"`
	Output     string  `json:"output"`
	Truncated  bool    `json:"truncated"`
	Error      string  `json:"error,omitempty"`
	StartedAt  *string `json:"started_at,omitempty"`
	FinishedAt *string `json:"finished_at,omitempty"`
}

func toBatchRunResponse(run model.BatchRun, results []model.RunResult) batchRunResponse {
	resp := batchRunResponse{
		Run: batchRunInfo{
			ID:        run.ID,
			Script:    run.Script,
			Status:    string(run.Status),
			CreatedAt: run.CreatedAt.Format(timeFormat),
		},
		Results: make([]runResultInfo, 0, len(results)),
	}
	for _, r := range results {
		resp.Results = append(resp.Results, runResultInfo{
			ServerID:   r.ServerID,
			Status:     string(r.Status),
			ExitCode:   r.ExitCode,
			Output:     r.Output,
			Truncated:  r.Truncated,
			Error:      r.Error,
			StartedAt:  formatTimePtr(r.StartedAt),
			FinishedAt: formatTimePtr(r.FinishedAt),
		})
	}
	return resp
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(timeFormat)
	return &s
}

// create dispatches a new Batch Run. A dispatch while the current run is
// still running is rejected with a conflict.
func (h *batchRunHandler) create(w http.ResponseWriter, r *http.Request) {
	var req batchRunRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if strings.TrimSpace(req.Script) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", "script is required")
		return
	}
	if len(req.ServerIDs) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", "server_ids is required")
		return
	}

	run, results, err := h.service.Dispatch(r.Context(), req.Script, dedupeStrings(req.ServerIDs))
	if err != nil {
		if errors.Is(err, model.ErrBatchRunInProgress) {
			httpx.WriteError(w, http.StatusConflict, "batch_run_in_progress", "Another batch run is already in progress")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to dispatch batch run")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toBatchRunResponse(run, results))
}

// current returns the most recent Batch Run and its per-Server results, or
// 404 when no run has ever been dispatched.
func (h *batchRunHandler) current(w http.ResponseWriter, r *http.Request) {
	run, results, err := h.service.Current(r.Context())
	if err != nil {
		if errors.Is(err, model.ErrBatchRunNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "No batch run has been dispatched yet")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to load batch run")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toBatchRunResponse(run, results))
}

// cancel terminates the processes on all target Servers of the running run.
// Idempotent: cancelling when nothing is running is a no-op.
func (h *batchRunHandler) cancel(w http.ResponseWriter, r *http.Request) {
	h.service.Cancel()
	w.WriteHeader(http.StatusNoContent)
}

// serveWS bridges the running phase over a dedicated WebSocket: per-Server
// output and status frames flow out, interactive input frames flow in.
func (h *batchRunHandler) serveWS(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	events, unsubscribe, err := h.service.Subscribe(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer unsubscribe()

	conn, err := batchWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Events → client. The service's Event is the wire format, so frames are
	// marshaled directly. Only this goroutine writes to the connection. When
	// the run completes the service closes the stream, which closes the
	// connection and signals the client to resync over REST.
	go func() {
		defer conn.Close()
		for ev := range events {
			data, err := json.Marshal(ev)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	// Client → service (interactive input). Stops on connection close.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg wsInput
		if json.Unmarshal(data, &msg) == nil && msg.Type == "input" && msg.ServerID != "" {
			_ = h.service.Input(runID, msg.ServerID, []byte(msg.Data))
		}
	}
}

// dedupeStrings removes duplicate server IDs while preserving order.
func dedupeStrings(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
