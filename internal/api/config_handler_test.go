package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"belochka/internal/api"
	"belochka/internal/config"
	"belochka/internal/hub"
)

func newConfigRouter(store api.ConfigStore) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithConfigStore(store))
}

// TestGetConfigReturnsAllFields verifies GET /api/config returns all config fields.
func TestGetConfigReturnsAllFields(t *testing.T) {
	cfg := config.Config{
		Port:             8080,
		DataDir:          "/data",
		Language:         "en",
		LogPath:          "/var/log/belochka.log",
		LogRetentionDays: 7,
	}
	store := config.NewStore(cfg, "")
	router := newConfigRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["port"] != float64(8080) {
		t.Errorf("port: got %v, want 8080", body["port"])
	}
	if body["data_dir"] != "/data" {
		t.Errorf("data_dir: got %v, want /data", body["data_dir"])
	}
	if body["language"] != "en" {
		t.Errorf("language: got %v, want en", body["language"])
	}
	if body["log_path"] != "/var/log/belochka.log" {
		t.Errorf("log_path: got %v, want /var/log/belochka.log", body["log_path"])
	}
	if body["log_retention_days"] != float64(7) {
		t.Errorf("log_retention_days: got %v, want 7", body["log_retention_days"])
	}
	// encryption_key must never appear in the response.
	if _, ok := body["encryption_key"]; ok {
		t.Error("encryption_key must not appear in GET /api/config response")
	}
}

// TestPatchConfigMergesPartialBody verifies that unset fields keep their current values.
func TestPatchConfigMergesPartialBody(t *testing.T) {
	cfg := config.Config{Port: 8080, DataDir: "/data", LogRetentionDays: 3}
	store := config.NewStore(cfg, "")
	router := newConfigRouter(store)

	body := `{"log_retention_days": 14}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["port"] != float64(8080) {
		t.Errorf("port should be unchanged: got %v, want 8080", resp["port"])
	}
	if resp["data_dir"] != "/data" {
		t.Errorf("data_dir should be unchanged: got %v, want /data", resp["data_dir"])
	}
	if resp["log_retention_days"] != float64(14) {
		t.Errorf("log_retention_days: got %v, want 14", resp["log_retention_days"])
	}
}

// TestPatchConfigPersistsToFile verifies the updated config is written to config.json.
func TestPatchConfigPersistsToFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{Port: 8080, DataDir: "/data", LogRetentionDays: 3}
	store := config.NewStore(cfg, configPath)
	router := newConfigRouter(store)

	body := `{"log_retention_days": 30}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	var fileCfg config.Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		t.Fatalf("config file is not valid JSON: %v", err)
	}
	if fileCfg.LogRetentionDays != 30 {
		t.Errorf("file log_retention_days: got %d, want 30", fileCfg.LogRetentionDays)
	}
	if fileCfg.Port != 8080 {
		t.Errorf("file port: got %d, want 8080", fileCfg.Port)
	}
}

// TestPatchConfigRestartRequiredOnPortChange verifies restart_required when port changes.
func TestPatchConfigRestartRequiredOnPortChange(t *testing.T) {
	cfg := config.Config{Port: 8080, DataDir: "/data", LogRetentionDays: 3}
	store := config.NewStore(cfg, "")
	router := newConfigRouter(store)

	body := `{"port": 9000}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["restart_required"] != true {
		t.Errorf("expected restart_required=true for port change, got %v", resp["restart_required"])
	}
}

// TestPatchConfigRestartRequiredOnDataDirChange verifies restart_required when data_dir changes.
func TestPatchConfigRestartRequiredOnDataDirChange(t *testing.T) {
	cfg := config.Config{Port: 8080, DataDir: "/data", LogRetentionDays: 3}
	store := config.NewStore(cfg, "")
	router := newConfigRouter(store)

	body := `{"data_dir": "/newdata"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["restart_required"] != true {
		t.Errorf("expected restart_required=true for data_dir change, got %v", resp["restart_required"])
	}
}

// TestPatchConfigNoRestartForLogRetentionDays verifies no restart_required for log_retention_days change.
func TestPatchConfigNoRestartForLogRetentionDays(t *testing.T) {
	cfg := config.Config{Port: 8080, DataDir: "/data", LogRetentionDays: 3}
	store := config.NewStore(cfg, "")
	router := newConfigRouter(store)

	body := `{"log_retention_days": 7}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if restart, ok := resp["restart_required"]; ok && restart == true {
		t.Errorf("expected no restart_required for log_retention_days change")
	}
}

// TestPatchConfigInvalidJSON returns 400 for malformed body.
func TestPatchConfigInvalidJSON(t *testing.T) {
	cfg := config.Config{Port: 8080}
	store := config.NewStore(cfg, "")
	router := newConfigRouter(store)

	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader("{bad json"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
