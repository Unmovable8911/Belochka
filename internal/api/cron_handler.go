package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

type createCronRequest struct {
	Minute     string `json:"minute"`
	Hour       string `json:"hour"`
	DayOfMonth string `json:"dayOfMonth"`
	Month      string `json:"month"`
	DayOfWeek  string `json:"dayOfWeek"`
	Command    string `json:"command"`
}

func (h *cronHandler) createCron(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req createCronRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	if strings.TrimSpace(req.Command) == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "command is required")
		return
	}

	// Read existing crontab.
	existing, err := h.executor.Execute(r.Context(), id, "crontab -l 2>/dev/null || true")
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to read crontab: "+err.Error())
		return
	}

	// Build new entry and append to existing content.
	entry := cron.CronEntry{
		Minute:     req.Minute,
		Hour:       req.Hour,
		DayOfMonth: req.DayOfMonth,
		Month:      req.Month,
		DayOfWeek:  req.DayOfWeek,
		Command:    req.Command,
		Enabled:    true,
	}
	newLine := cron.BuildCronLine(entry)
	content := strings.TrimRight(existing, "\n")
	if content != "" {
		content += "\n"
	}
	content += newLine + "\n"

	// Write back using base64 to avoid shell escaping issues with arbitrary content.
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	writeCmd := fmt.Sprintf("echo %s | base64 -d | crontab -", encoded)
	if _, err := h.executor.Execute(r.Context(), id, writeCmd); err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to write crontab: "+err.Error())
		return
	}

	entry.Raw = newLine
	writeJSON(w, http.StatusCreated, entry)
}

type updateCronRequest struct {
	Minute     string `json:"minute"`
	Hour       string `json:"hour"`
	DayOfMonth string `json:"dayOfMonth"`
	Month      string `json:"month"`
	DayOfWeek  string `json:"dayOfWeek"`
	Command    string `json:"command"`
	Enabled    bool   `json:"enabled"`
}

func (h *cronHandler) updateCron(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idx, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || idx < 0 {
		writeError(w, http.StatusBadRequest, "invalid_index", "index must be a non-negative integer")
		return
	}

	var req updateCronRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	existing, err := h.executor.Execute(r.Context(), id, "crontab -l 2>/dev/null || true")
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to read crontab: "+err.Error())
		return
	}

	parsed := cron.ParseCrontab(existing)
	if idx >= len(parsed.Entries) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("cron entry %d does not exist", idx))
		return
	}

	entry := cron.CronEntry{
		Minute:     req.Minute,
		Hour:       req.Hour,
		DayOfMonth: req.DayOfMonth,
		Month:      req.Month,
		DayOfWeek:  req.DayOfWeek,
		Command:    req.Command,
		Enabled:    req.Enabled,
	}

	newContent := cron.ReplaceCronEntry(existing, idx, &entry)
	encoded := base64.StdEncoding.EncodeToString([]byte(newContent))
	writeCmd := fmt.Sprintf("echo %s | base64 -d | crontab -", encoded)
	if _, err := h.executor.Execute(r.Context(), id, writeCmd); err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to write crontab: "+err.Error())
		return
	}

	entry.Raw = cron.BuildLine(entry)
	writeJSON(w, http.StatusOK, entry)
}

func (h *cronHandler) deleteCron(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idx, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || idx < 0 {
		writeError(w, http.StatusBadRequest, "invalid_index", "index must be a non-negative integer")
		return
	}

	existing, err := h.executor.Execute(r.Context(), id, "crontab -l 2>/dev/null || true")
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to read crontab: "+err.Error())
		return
	}

	parsed := cron.ParseCrontab(existing)
	if idx >= len(parsed.Entries) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("cron entry %d does not exist", idx))
		return
	}

	newContent := cron.ReplaceCronEntry(existing, idx, nil)
	encoded := base64.StdEncoding.EncodeToString([]byte(newContent))
	writeCmd := fmt.Sprintf("echo %s | base64 -d | crontab -", encoded)
	if _, err := h.executor.Execute(r.Context(), id, writeCmd); err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to write crontab: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
