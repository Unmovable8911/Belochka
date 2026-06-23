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

	if cfg.EncryptionKey != "" {
		t.Errorf("default encryption_key = %q, want empty", cfg.EncryptionKey)
	}
}

func TestLoadFromYAMLFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "belochka.yaml")

	content := []byte("port: 8080\ndata_dir: /var/lib/belochka\nencryption_key: my-secret-key\n")
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
		t.Errorf("data_dir = %q, want %q", cfg.DataDir, "/var/lib/belochka")
	}

	if cfg.EncryptionKey != "my-secret-key" {
		t.Errorf("encryption_key = %q, want %q", cfg.EncryptionKey, "my-secret-key")
	}
}

func TestPartialYAMLUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "belochka.yaml")

	// Only set port; data_dir and encryption_key should use defaults.
	content := []byte("port: 9999\n")
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

	if cfg.EncryptionKey != "" {
		t.Errorf("encryption_key = %q, want empty default", cfg.EncryptionKey)
	}
}

func TestExplicitPathNotFoundIsError(t *testing.T) {
	_, err := Load("/nonexistent/path/belochka.yaml")
	if err == nil {
		t.Fatal("expected error when explicit config path does not exist")
	}
}

func TestEnvVarOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "belochka.yaml")

	content := []byte("encryption_key: from-file\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("BELOCHKA_ENCRYPTION_KEY", "from-env")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.EncryptionKey != "from-env" {
		t.Errorf("encryption_key = %q, want %q (env should override file)", cfg.EncryptionKey, "from-env")
	}
}

func TestEnvVarWithNoConfigFile(t *testing.T) {
	t.Setenv("BELOCHKA_ENCRYPTION_KEY", "env-only-key")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.EncryptionKey != "env-only-key" {
		t.Errorf("encryption_key = %q, want %q", cfg.EncryptionKey, "env-only-key")
	}
}

func TestCWDFallback(t *testing.T) {
	// Create a temp dir with a belochka.yaml and chdir into it.
	dir := t.TempDir()
	content := []byte("port: 7777\n")
	if err := os.WriteFile(filepath.Join(dir, "belochka.yaml"), content, 0644); err != nil {
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

func TestInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "belochka.yaml")

	content := []byte("port: [invalid yaml\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
