package api

import (
	"encoding/json"
	"io"
	"net/http"

	"belochka/internal/config"
	"belochka/internal/httpx"
)

// configHandler handles GET and PATCH /api/config.
type configHandler struct {
	store            config.ConfigStore
	onLanguageChange func(string)
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
	httpx.WriteJSON(w, http.StatusOK, cfg)
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Failed to read request body")
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if _, ok := raw["password_hash"]; ok {
		httpx.WriteError(w, http.StatusBadRequest, "forbidden_field", "password_hash cannot be modified through this endpoint")
		return
	}

	var body patchBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	old := h.store.Get()

	if body.Port != nil && (*body.Port < 1 || *body.Port > 65535) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "port must be between 1 and 65535")
		return
	}
	if body.LogRetentionDays != nil && (*body.LogRetentionDays < 0 || *body.LogRetentionDays > 365) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "log retention days must be between 0 and 365")
		return
	}

	var updated config.Config
	if err := h.store.Update(func(c *config.Config) {
		if body.Port != nil {
			c.Port = *body.Port
		}
		if body.DataDir != nil {
			c.DataDir = *body.DataDir
		}
		if body.Language != nil {
			c.Language = *body.Language
			if h.onLanguageChange != nil {
				h.onLanguageChange(*body.Language)
			}
		}
		if body.LogPath != nil {
			c.LogPath = *body.LogPath
		}
		if body.LogRetentionDays != nil {
			c.LogRetentionDays = *body.LogRetentionDays
		}
		updated = *c
	}); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "persist_error", "Failed to save config")
		return
	}

	restartRequired := updated.Port != old.Port || updated.DataDir != old.DataDir

	httpx.WriteJSON(w, http.StatusOK, patchConfigResponse{
		Config:          updated,
		RestartRequired: restartRequired,
	})
}
