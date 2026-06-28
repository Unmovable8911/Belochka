package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store holds shared mutable config state, safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	cfg  Config
	path string
}

// NewStore creates a Store initialised with cfg. If path is non-empty,
// Set atomically persists the config to that file; otherwise Set is
// in-memory only.
func NewStore(cfg Config, path string) *Store {
	return &Store{cfg: cfg, path: path}
}

// Get returns the current in-memory config.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Set updates the in-memory config and, when a path is configured,
// atomically writes it to disk before updating the in-memory value.
func (s *Store) Set(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path != "" {
		if err := atomicWriteConfig(s.path, cfg); err != nil {
			return err
		}
	}
	s.cfg = cfg
	return nil
}

// Language returns the current language setting.
func (s *Store) Language() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Language
}

// SetLanguage updates the language field in the config, persisting to disk
// when a path is configured.
func (s *Store) SetLanguage(lang string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Language = lang
	if s.path != "" {
		return atomicWriteConfig(s.path, s.cfg)
	}
	return nil
}

// atomicWriteConfig marshals cfg to JSON and writes it to path via a
// temp-file + rename so the update is atomic on POSIX systems.
func atomicWriteConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
