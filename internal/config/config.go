package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the application configuration.
type Config struct {
	Port             int    `json:"port"`
	DataDir          string `json:"data_dir"`
	Language         string `json:"language"`
	LogPath          string `json:"log_path"`
	LogRetentionDays int    `json:"log_retention_days"`
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
// it tries "./config.json" in the current working directory. If no file
// is found, built-in defaults are returned.
// BELOCHKA_ENCRYPTION_KEY is not stored in Config; consumers read it directly
// from the environment.
func Load(path string) (Config, error) {
	cfg := defaults()

	filePath := path
	if filePath == "" {
		filePath = "config.json"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && path == "" {
			// CWD fallback file not found is fine — use defaults.
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}
