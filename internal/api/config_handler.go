package api

import (
	"encoding/json"
	"net/http"

	"belochka/internal/config"
)

// ConfigStore provides thread-safe read/write access to the application config.
type ConfigStore interface {
	Get() config.Config
	Set(config.Config) error
}

// configHandler handles GET and PATCH /api/config.
type configHandler struct {
	store ConfigStore
}

func (h *configHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Get())
}

// patchBody holds optional fields for a config update. Pointer fields let
// the handler distinguish "provided" from "zero / not sent".
type patchBody struct {
	Port             *int    `json:"port"`
	DataDir          *string `json:"data_dir"`
	Language         *string `json:"language"`
	LogPath          *string `json:"log_path"`
	LogRetentionDays *int    `json:"log_retention_days"`
}

type patchConfigResponse struct {
	config.Config
	RestartRequired bool `json:"restart_required,omitempty"`
}

func (h *configHandler) patchConfig(w http.ResponseWriter, r *http.Request) {
	var body patchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	old := h.store.Get()
	updated := old

	if body.Port != nil {
		updated.Port = *body.Port
	}
	if body.DataDir != nil {
		updated.DataDir = *body.DataDir
	}
	if body.Language != nil {
		updated.Language = *body.Language
	}
	if body.LogPath != nil {
		updated.LogPath = *body.LogPath
	}
	if body.LogRetentionDays != nil {
		updated.LogRetentionDays = *body.LogRetentionDays
	}

	if err := h.store.Set(updated); err != nil {
		writeError(w, http.StatusInternalServerError, "persist_error", "Failed to save config")
		return
	}

	restartRequired := updated.Port != old.Port || updated.DataDir != old.DataDir

	writeJSON(w, http.StatusOK, patchConfigResponse{
		Config:          updated,
		RestartRequired: restartRequired,
	})
}
