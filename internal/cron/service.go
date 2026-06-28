package cron

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ErrCronIndexOutOfRange is returned when a cron entry index does not exist.
var ErrCronIndexOutOfRange = errors.New("cron entry index out of range")

// Executor runs shell commands on a remote server.
type Executor interface {
	Execute(ctx context.Context, serverID, cmd string) (string, error)
}

// Runner executes a command and returns combined stdout+stderr output and the
// exit code. Unlike Executor, a non-zero exit code is not an error.
type Runner interface {
	RunCommand(ctx context.Context, serverID, cmd string) (output string, exitCode int, err error)
}

// Service orchestrates reading and writing crontabs on remote servers.
type Service struct {
	executor Executor
	runner   Runner
}

// NewService creates a Service backed by the given executor and runner.
func NewService(executor Executor, runner Runner) *Service {
	return &Service{executor: executor, runner: runner}
}

// readCrontab reads the remote crontab. The "|| true" ensures exit 0 even when
// no crontab exists.
func (s *Service) readCrontab(ctx context.Context, serverID string) (string, error) {
	return s.executor.Execute(ctx, serverID, "crontab -l 2>/dev/null || true")
}

// writeCrontab writes content as the remote crontab, using base64 to avoid
// shell escaping issues with arbitrary content.
func (s *Service) writeCrontab(ctx context.Context, serverID, content string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	writeCmd := fmt.Sprintf("echo %s | base64 -d | crontab -", encoded)
	_, err := s.executor.Execute(ctx, serverID, writeCmd)
	return err
}

// List reads and parses the remote crontab.
func (s *Service) List(ctx context.Context, serverID string) (CronResult, error) {
	output, err := s.readCrontab(ctx, serverID)
	if err != nil {
		return CronResult{}, err
	}
	return ParseCrontab(output), nil
}

// Run executes the command of the cron entry at index and returns its combined
// output and exit code. Returns ErrCronIndexOutOfRange when index is out of
// range.
func (s *Service) Run(ctx context.Context, serverID string, index int) (output string, exitCode int, err error) {
	parsed, err := s.List(ctx, serverID)
	if err != nil {
		return "", 0, err
	}
	if index >= len(parsed.Entries) {
		return "", 0, ErrCronIndexOutOfRange
	}
	return s.runner.RunCommand(ctx, serverID, parsed.Entries[index].Command)
}

// Create appends a new (enabled) cron entry and returns the created entry with
// its Raw line set.
func (s *Service) Create(ctx context.Context, serverID string, entry CronEntry) (CronEntry, error) {
	existing, err := s.readCrontab(ctx, serverID)
	if err != nil {
		return CronEntry{}, err
	}

	entry.Enabled = true
	newLine := BuildCronLine(entry)
	content := strings.TrimRight(existing, "\n")
	if content != "" {
		content += "\n"
	}
	content += newLine + "\n"

	if err := s.writeCrontab(ctx, serverID, content); err != nil {
		return CronEntry{}, err
	}

	entry.Raw = newLine
	return entry, nil
}

// Update replaces the cron entry at index and returns the updated entry with its
// Raw line set. Returns ErrCronIndexOutOfRange when index is out of range.
func (s *Service) Update(ctx context.Context, serverID string, index int, entry CronEntry) (CronEntry, error) {
	existing, err := s.readCrontab(ctx, serverID)
	if err != nil {
		return CronEntry{}, err
	}

	parsed := ParseCrontab(existing)
	if index >= len(parsed.Entries) {
		return CronEntry{}, ErrCronIndexOutOfRange
	}

	newContent := ReplaceCronEntry(existing, index, &entry)
	if err := s.writeCrontab(ctx, serverID, newContent); err != nil {
		return CronEntry{}, err
	}

	entry.Raw = BuildLine(entry)
	return entry, nil
}

// Delete removes the cron entry at index. Returns ErrCronIndexOutOfRange when
// index is out of range.
func (s *Service) Delete(ctx context.Context, serverID string, index int) error {
	existing, err := s.readCrontab(ctx, serverID)
	if err != nil {
		return err
	}

	parsed := ParseCrontab(existing)
	if index >= len(parsed.Entries) {
		return ErrCronIndexOutOfRange
	}

	newContent := ReplaceCronEntry(existing, index, nil)
	return s.writeCrontab(ctx, serverID, newContent)
}
