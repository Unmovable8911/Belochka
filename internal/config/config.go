package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Port          int    `yaml:"port"`
	DataDir       string `yaml:"data_dir"`
	EncryptionKey string `yaml:"encryption_key"`
}

// defaults returns a Config with sensible default values.
func defaults() Config {
	return Config{
		Port:    53136,
		DataDir: "./data",
	}
}

// Load reads configuration from the given file path. If path is empty,
// it tries "./belochka.yaml" in the current working directory. If no file
// is found, built-in defaults are returned. Environment variable
// BELOCHKA_ENCRYPTION_KEY overrides the config file value.
func Load(path string) (Config, error) {
	cfg := defaults()

	filePath := path
	if filePath == "" {
		filePath = "belochka.yaml"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && path == "" {
			// CWD fallback file not found is fine — use defaults.
			return applyEnv(cfg), nil
		}
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}

	return applyEnv(cfg), nil
}

// applyEnv overrides config values with environment variables.
func applyEnv(cfg Config) Config {
	if v := os.Getenv("BELOCHKA_ENCRYPTION_KEY"); v != "" {
		cfg.EncryptionKey = v
	}
	return cfg
}
