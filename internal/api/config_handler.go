package api

import (
	"encoding/json"
	"io"
	"net/http"

	"belochka/internal/config"
)

// ConfigStore provides thread-safe read/write access to the application config.
type ConfigStore interface {
	Get() config.Config
	Set(config.Config) error
	BaseDir() string
}

// configHandler handles GET and PATCH /api/config.
type configHandler struct {
	store ConfigStore
}

func (h *configHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	baseDir := h.store.BaseDir()
	if cfg.LogPath == "" {
		cfg.LogPath = config.LogFilePath(baseDir)
	} else {
		cfg.LogPath = config.ResolvePath(cfg.LogPath, baseDir)
	}
	cfg.DataDir = config.ResolvePath(cfg.DataDir, baseDir)
	cfg.PasswordHash = "" // Never expose the password hash.
	writeJSON(w, http.StatusOK, cfg)
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
	// Reject any attempt to modify the password hash through the config API.
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to read request body")
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if _, ok := raw["password_hash"]; ok {
		writeError(w, http.StatusBadRequest, "forbidden_field", "password_hash cannot be modified through this endpoint")
		return
	}

	var body patchBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
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

	if updated.Port < 1 || updated.Port > 65535 {
		writeError(w, http.StatusBadRequest, "invalid_input", "port must be between 1 and 65535")
		return
	}
	if updated.LogRetentionDays < 0 || updated.LogRetentionDays > 365 {
		writeError(w, http.StatusBadRequest, "invalid_input", "log retention days must be between 0 and 365")
		return
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
