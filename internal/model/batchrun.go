package model

import (
	"errors"
	"time"
)

// ErrBatchRunInProgress is returned when a dispatch is attempted while the
// current Batch Run is still running. Runs are serialized by design.
var ErrBatchRunInProgress = errors.New("batch run already in progress")

// ErrBatchRunNotFound is returned by the store when no Batch Run has ever been
// dispatched (or its records were replaced by a newer run).
var ErrBatchRunNotFound = errors.New("batch run not found")

// BatchRunStatus is the status of a Batch Run.
type BatchRunStatus string

const (
	BatchRunRunning BatchRunStatus = "running"
	BatchRunDone    BatchRunStatus = "done"
)

// RunResultStatus is the status of one Server's execution within a Batch Run.
// Note this is a different layer from the Connection State: `failed` here means
// the execution failed (non-zero exit, connection error, or cancelled), not
// that the Server's SSH connection is down.
type RunResultStatus string

const (
	ResultPending RunResultStatus = "pending"
	ResultRunning RunResultStatus = "running"
	ResultSuccess RunResultStatus = "success"
	ResultFailed  RunResultStatus = "failed"
)

// BatchRun is a single dispatch of one script to a set of Servers. Only the
// most recent Run is retained — a new dispatch replaces the previous one.
type BatchRun struct {
	ID        string
	Script    string
	Status    BatchRunStatus
	CreatedAt time.Time
}

// RunResult is one Server's outcome within a Batch Run. The Output is a single
// merged stream (stdout+stderr are not separable over a PTY), capped at a fixed
// limit and marked Truncated when the cap was hit.
type RunResult struct {
	RunID      string
	ServerID   string
	Status     RunResultStatus
	ExitCode   *int
	Output     string
	Truncated  bool
	Error      string
	StartedAt  *time.Time
	FinishedAt *time.Time
}
