package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ConfigStore is the canonical read/write interface for application configuration.
// The concrete [Store] type satisfies it; consumers should depend on this
// interface rather than defining their own subsets.
type ConfigStore interface {
	Get() Config
	Update(func(*Config)) error
	Language() string
	PasswordHash() string
	BaseDir() string
}

// Store holds shared mutable config state, safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	cfg     Config
	path    string
	baseDir string
}

// NewStore creates a Store initialised with cfg. If path is non-empty,
// Set atomically persists the config to that file; otherwise Set is
// in-memory only. baseDir is the directory of the running binary, used
// to resolve relative paths at runtime.
func NewStore(cfg Config, path string, baseDir string) *Store {
	return &Store{cfg: cfg, path: path, baseDir: baseDir}
}

// BaseDir returns the binary directory used for resolving relative paths.
func (s *Store) BaseDir() string {
	return s.baseDir
}

// Get returns the current in-memory config.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Update calls fn with a pointer to the in-memory config while holding
// the write lock. When fn returns, the updated config is atomically
// persisted to disk (if a path is configured). Persist errors are
// returned; the in-memory value is updated regardless.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	if s.path != "" {
		if err := atomicWriteConfig(s.path, s.cfg); err != nil {
			return err
		}
	}
	return nil
}

// Language returns the current language setting.
func (s *Store) Language() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Language
}

// PasswordHash returns the current password hash.
func (s *Store) PasswordHash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.PasswordHash
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
