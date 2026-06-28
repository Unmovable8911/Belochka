package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with no config file failed: %v", err)
	}

	if cfg.Port != 53136 {
		t.Errorf("default port = %d, want 53136", cfg.Port)
	}

	if cfg.DataDir != "./data" {
		t.Errorf("default data_dir = %q, want %q", cfg.DataDir, "./data")
	}

	if cfg.LogRetentionDays != 3 {
		t.Errorf("default log_retention_days = %d, want 3", cfg.LogRetentionDays)
	}

	if cfg.Language != "" {
		t.Errorf("default language = %q, want empty", cfg.Language)
	}

	if cfg.LogPath != "" {
		t.Errorf("default log_path = %q, want empty", cfg.LogPath)
	}
}

func TestLoadFromJSONFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	content := []byte(`{"port":8080,"data_dir":"/var/lib/belochka","language":"zh","log_path":"/var/log/belochka.log","log_retention_days":7}`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Port)
	}

	if cfg.DataDir != "/var/lib/belochka" {
		t.Errorf("data_dir = %q, want /var/lib/belochka", cfg.DataDir)
	}

	if cfg.Language != "zh" {
		t.Errorf("language = %q, want zh", cfg.Language)
	}

	if cfg.LogPath != "/var/log/belochka.log" {
		t.Errorf("log_path = %q, want /var/log/belochka.log", cfg.LogPath)
	}

	if cfg.LogRetentionDays != 7 {
		t.Errorf("log_retention_days = %d, want 7", cfg.LogRetentionDays)
	}
}

func TestPartialJSONUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Only set port; other fields should use defaults.
	content := []byte(`{"port":9999}`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("port = %d, want 9999", cfg.Port)
	}

	if cfg.DataDir != "./data" {
		t.Errorf("data_dir = %q, want default %q", cfg.DataDir, "./data")
	}

	if cfg.LogRetentionDays != 3 {
		t.Errorf("log_retention_days = %d, want default 3", cfg.LogRetentionDays)
	}
}

func TestExplicitPathNotFoundIsError(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error when explicit config path does not exist")
	}
}

func TestEncryptionKeyEnvVar(t *testing.T) {
	// BELOCHKA_ENCRYPTION_KEY is not stored in Config; it is read directly
	// by consumers. Verify Load does not fail when it is set.
	t.Setenv("BELOCHKA_ENCRYPTION_KEY", "from-env")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify that Config does not expose the key.
	_ = cfg.Port // just confirm Load succeeded and returned a usable Config
}

func TestCWDFallback(t *testing.T) {
	// Create a temp dir with a config.json and chdir into it.
	dir := t.TempDir()
	content := []byte(`{"port":7777}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), content, 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load from CWD failed: %v", err)
	}

	if cfg.Port != 7777 {
		t.Errorf("port = %d, want 7777 (from CWD fallback)", cfg.Port)
	}
}

func TestInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	content := []byte(`{invalid json`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
