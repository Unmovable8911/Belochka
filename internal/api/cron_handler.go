package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"belochka/internal/cron"

	"github.com/go-chi/chi/v5"
)

// cronHandler handles cron endpoints.
type cronHandler struct {
	service *cron.Service
}

func (h *cronHandler) listCrons(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	result, err := h.service.List(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to read crontab: "+err.Error())
		return
	}

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

	if err := validateCronFields(req.Minute, req.Hour, req.DayOfMonth, req.Month, req.DayOfWeek); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	entry := cron.CronEntry{
		Minute:     req.Minute,
		Hour:       req.Hour,
		DayOfMonth: req.DayOfMonth,
		Month:      req.Month,
		DayOfWeek:  req.DayOfWeek,
		Command:    req.Command,
	}

	created, err := h.service.Create(r.Context(), id, entry)
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to write crontab: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
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

	if err := validateCronFields(req.Minute, req.Hour, req.DayOfMonth, req.Month, req.DayOfWeek); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
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

	updated, err := h.service.Update(r.Context(), id, idx, entry)
	if err != nil {
		if errors.Is(err, cron.ErrCronIndexOutOfRange) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("cron entry %d does not exist", idx))
			return
		}
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to write crontab: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

type runCronResponse struct {
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
}

func (h *cronHandler) runCron(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idx, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || idx < 0 {
		writeError(w, http.StatusBadRequest, "invalid_index", "index must be a non-negative integer")
		return
	}

	output, exitCode, err := h.service.Run(r.Context(), id, idx)
	if err != nil {
		if errors.Is(err, cron.ErrCronIndexOutOfRange) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("cron entry %d does not exist", idx))
			return
		}
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to execute command: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runCronResponse{ExitCode: exitCode, Output: output})
}

func (h *cronHandler) deleteCron(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idx, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || idx < 0 {
		writeError(w, http.StatusBadRequest, "invalid_index", "index must be a non-negative integer")
		return
	}

	if err := h.service.Delete(r.Context(), id, idx); err != nil {
		if errors.Is(err, cron.ErrCronIndexOutOfRange) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("cron entry %d does not exist", idx))
			return
		}
		writeError(w, http.StatusBadGateway, "ssh_error", "Failed to write crontab: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validateCronFields checks that all required cron schedule fields are non-empty.
func validateCronFields(minute, hour, dayOfMonth, month, dayOfWeek string) error {
	if minute == "" {
		return fmt.Errorf("minute is required")
	}
	if hour == "" {
		return fmt.Errorf("hour is required")
	}
	if dayOfMonth == "" {
		return fmt.Errorf("dayOfMonth is required")
	}
	if month == "" {
		return fmt.Errorf("month is required")
	}
	if dayOfWeek == "" {
		return fmt.Errorf("dayOfWeek is required")
	}
	return nil
}
