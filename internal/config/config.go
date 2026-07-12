package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration.
type Config struct {
	Port             int    `json:"port"`
	DataDir          string `json:"data_dir"`
	Language         string `json:"language"`
	LogPath          string `json:"log_path"`
	LogRetentionDays int    `json:"log_retention_days"`
	PasswordHash     string `json:"password_hash,omitempty"`
}

// ResolvePath resolves a relative path against baseDir. If baseDir is empty,
// it falls back to resolving against the current working directory.
// Absolute paths are returned unchanged (cleaned).
func ResolvePath(path string, baseDir string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if baseDir != "" {
		return filepath.Join(baseDir, path)
	}
	abs, _ := filepath.Abs(path)
	return abs
}

// ConfigFilePath returns the resolved config file path. When path is
// non-empty, it is returned as-is (the caller supplied an explicit path).
// Otherwise it returns "config.json" under baseDir, or just "config.json"
// when baseDir is also empty (CWD fallback).
func ConfigFilePath(path string, baseDir string) string {
	if path != "" {
		return path
	}
	if baseDir != "" {
		return filepath.Join(baseDir, "config.json")
	}
	return "config.json"
}

// LogFilePath returns the default log file path under baseDir.
// When baseDir is empty (CWD fallback), it returns "belochka.log" in the
// current working directory.
func LogFilePath(baseDir string) string {
	if baseDir != "" {
		return filepath.Join(baseDir, "belochka.log")
	}
	return "belochka.log"
}

// defaults returns a Config with sensible default values.
func defaults() Config {
	return Config{
		Port:             53136,
		DataDir:          "./data",
		LogRetentionDays: 3,
	}
}

// Load reads configuration from the given file path. If path is empty,
// it looks for "config.json" under baseDir. If baseDir is also empty,
// it falls back to "./config.json" in the current working directory.
// If path is explicitly provided, it is used as-is (relative to CWD,
// per Unix CLI convention). When no file is found, built-in defaults
// are returned.
// BELOCHKA_ENCRYPTION_KEY is not stored in Config; consumers read it directly
// from the environment.
func Load(path string, baseDir string) (Config, error) {
	cfg := defaults()

	filePath := ConfigFilePath(path, baseDir)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && path == "" {
			// Default file not found is fine — use defaults.
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}
