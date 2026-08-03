// Package batchrun dispatches one script to a set of Servers and tracks each
// Server's execution as a Run Result. Runs are serialized (only one at a
// time) and non-historical (a new dispatch replaces the previous run).
package batchrun

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"belochka/internal/model"
	"belochka/internal/pty"
	"belochka/internal/ssh"

	"github.com/google/uuid"
)

// PTYOpener opens a PTY session running a command on a Server. Satisfied by
// *pty.Opener.
type PTYOpener interface {
	OpenCommand(serverID, command string, rows, cols int) (pty.Session, error)
}

// Store defines the persistence operations required by the service.
type Store interface {
	ReplaceBatchRun(ctx context.Context, run model.BatchRun, results []model.RunResult) (model.BatchRun, error)
	GetBatchRun(ctx context.Context) (model.BatchRun, error)
	ListBatchRunResults(ctx context.Context, runID string) ([]model.RunResult, error)
	UpdateBatchRunResult(ctx context.Context, result model.RunResult) error
	SetBatchRunDone(ctx context.Context, runID string) error
}

// Event is a per-Server update streamed to WebSocket subscribers while a run
// is in progress. Type is "output" (Data holds a stream chunk) or "status"
// (Status holds the new Run Result status, with ExitCode/Truncated/Error for
// finished Servers). Event is also the batch run WebSocket wire format: the
// JSON tags below are the protocol contract the frontend parses, so the wire
// struct lives here at the seam, not duplicated in the handler.
type Event struct {
	Type       string                `json:"type"`
	ServerID   string                `json:"server_id"`
	Data       string                `json:"data,omitempty"`
	Status     model.RunResultStatus `json:"status,omitempty"`
	ExitCode   *int                  `json:"exit_code,omitempty"`
	Truncated  bool                  `json:"truncated,omitempty"`
	Error      string                `json:"error,omitempty"`
	FinishedAt *EventTime           `json:"finished_at,omitempty"`
}

// timeFormat is the whole-second RFC 3339 UTC timestamp format used on the
// wire, mirroring the REST API's format so both protocols read alike.
const timeFormat = "2006-01-02T15:04:05Z"

// EventTime marshals a time.Time as the batch run wire format (see
// timeFormat). time.Time's default RFC 3339Nano would emit fractional
// seconds, but the frontend compares finished_at lexicographically, so the
// timestamp must stay fixed-width. The field is *EventTime so nil (a result
// not yet finished) omits the key entirely.
type EventTime time.Time

func (t *EventTime) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("null"), nil
	}
	return []byte(`"` + time.Time(*t).Format(timeFormat) + `"`), nil
}

const (
	readBufSize    = 4096
	maxOutputBytes = 64 * 1024
)

// errCancelled marks results ended by a manual cancel.
const errCancelled = "cancelled"

// errInterrupted marks results finalized at startup because the process died
// mid-run (a running run cannot resume after a restart).
const errInterrupted = "interrupted by restart"

// runState holds the in-memory state of the current (only) Batch Run.
type runState struct {
	run      model.BatchRun
	sessions map[string]pty.Session // by Server ID; guarded by smu
	smu      sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	done     bool // set once all per-Server executions have finalized
}

// Service dispatches and tracks Batch Runs.
type Service struct {
	store  Store
	exec   ssh.Executor
	opener PTYOpener
	now    func() time.Time

	// flushInterval bounds how often streamed output is persisted while a run
	// is in progress; chunk writes aggregate to one store write per interval.
	flushInterval time.Duration

	mu      sync.Mutex
	current *runState

	subsMu sync.Mutex
	subs   map[chan Event]struct{}
}

// WithFlushInterval overrides how often streamed output is persisted while a
// run is in progress (default 500ms). Tests use it to make flush cadence
// deterministic.
func WithFlushInterval(d time.Duration) func(*Service) {
	return func(s *Service) { s.flushInterval = d }
}

// New creates a Service. Any run left in running state by a previous process
// is finalized as failed (it cannot resume), so a restart never blocks new
// dispatches.
func New(store Store, exec ssh.Executor, opener PTYOpener, opts ...func(*Service)) *Service {
	s := &Service{
		store:         store,
		exec:          exec,
		opener:        opener,
		now:           func() time.Time { return time.Now().UTC() },
		flushInterval: 500 * time.Millisecond,
		subs:          make(map[chan Event]struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.finalizeStaleRun()
	return s
}

// finalizeStaleRun marks a run left running by a previous process as done,
// with its unfinished results failed as interrupted.
func (s *Service) finalizeStaleRun() {
	ctx := context.Background()
	run, err := s.store.GetBatchRun(ctx)
	if errors.Is(err, model.ErrBatchRunNotFound) {
		return
	}
	if err != nil {
		slog.Error("batchrun: load current run", "error", err)
		return
	}
	if run.Status != model.BatchRunRunning {
		return
	}

	results, err := s.store.ListBatchRunResults(ctx, run.ID)
	if err != nil {
		slog.Error("batchrun: load stale results", "error", err)
		return
	}
	now := s.now()
	for i := range results {
		if results[i].Status == model.ResultPending || results[i].Status == model.ResultRunning {
			results[i].Status = model.ResultFailed
			results[i].Error = errInterrupted
			results[i].FinishedAt = &now
			if err := s.store.UpdateBatchRunResult(ctx, results[i]); err != nil {
				slog.Error("batchrun: finalize stale result", "server_id", results[i].ServerID, "error", err)
			}
		}
	}
	if err := s.store.SetBatchRunDone(ctx, run.ID); err != nil {
		slog.Error("batchrun: finalize stale run", "error", err)
	}
	slog.Info("batchrun: finalized stale run from previous process", "run_id", run.ID)
}

// Dispatch creates a new Batch Run for the given script and target Servers and
// starts one background execution per Server. Returns ErrBatchRunInProgress
// while the current run is still running.
func (s *Service) Dispatch(ctx context.Context, script string, serverIDs []string) (model.BatchRun, []model.RunResult, error) {
	// In-memory serialization guard: the store transaction is the source of
	// truth, but checking first keeps two concurrent dispatches from both
	// reaching the store while the current run is still running.
	s.mu.Lock()
	if s.current != nil && !s.current.done {
		s.mu.Unlock()
		return model.BatchRun{}, nil, model.ErrBatchRunInProgress
	}
	s.mu.Unlock()

	run, results := s.newRun(script, serverIDs)

	if _, err := s.store.ReplaceBatchRun(ctx, run, results); err != nil {
		return model.BatchRun{}, nil, err
	}

	rs := &runState{
		run:      run,
		sessions: make(map[string]pty.Session),
	}

	// Return copies taken before the goroutines start mutating the originals,
	// so the caller (and the API handler serializing the response) never
	// races with the background executions.
	copies := make([]model.RunResult, len(results))
	for i := range results {
		copies[i] = results[i]
	}

	runCtx, cancel := context.WithCancel(context.Background())
	rs.cancel = cancel

	s.mu.Lock()
	s.current = rs
	s.mu.Unlock()

	for i := range results {
		rs.wg.Add(1)
		go s.runOne(runCtx, rs, &results[i])
	}
	go s.waitForCompletion(rs)

	return run, copies, nil
}

// waitForCompletion marks the run done once every per-Server execution has
// finalized, then records it in the store and closes all subscriber channels.
// Closing the channels ends the WebSocket streams, which the frontend uses as
// its "run finished" signal to resync over REST.
func (s *Service) waitForCompletion(rs *runState) {
	rs.wg.Wait()
	if err := s.store.SetBatchRunDone(context.Background(), rs.run.ID); err != nil {
		slog.Error("batchrun: mark run done", "run_id", rs.run.ID, "error", err)
	}
	s.mu.Lock()
	rs.done = true
	s.mu.Unlock()

	s.subsMu.Lock()
	for ch := range s.subs {
		close(ch)
	}
	s.subs = make(map[chan Event]struct{})
	s.subsMu.Unlock()
}

// Current returns the most recent Batch Run and its per-Server results, or
// model.ErrBatchRunNotFound when no run has ever been dispatched.
func (s *Service) Current(ctx context.Context) (model.BatchRun, []model.RunResult, error) {
	run, err := s.store.GetBatchRun(ctx)
	if err != nil {
		return model.BatchRun{}, nil, err
	}
	results, err := s.store.ListBatchRunResults(ctx, run.ID)
	if err != nil {
		return model.BatchRun{}, nil, err
	}
	return run, results, nil
}

// Cancel terminates the processes on all target Servers of the running run
// and ends their results as failed with error "cancelled". No-op when no run
// is in progress.
func (s *Service) Cancel() {
	s.mu.Lock()
	rs := s.current
	done := rs == nil || rs.done
	s.mu.Unlock()
	if done {
		return
	}

	rs.cancel()
	rs.smu.Lock()
	for serverID, sess := range rs.sessions {
		if err := sess.Close(); err != nil {
			slog.Debug("batchrun: close session on cancel", "server_id", serverID, "error", err)
		}
	}
	rs.smu.Unlock()
}

// Input writes interactive stdin data to a running execution of the current
// run. Returns an error when the run is not active or the Server has no live
// session (e.g. its process already exited).
func (s *Service) Input(runID, serverID string, data []byte) error {
	s.mu.Lock()
	rs := s.current
	done := rs == nil || rs.done
	s.mu.Unlock()
	if done || rs.run.ID != runID {
		return errors.New("batch run is not active")
	}

	rs.smu.Lock()
	sess, ok := rs.sessions[serverID]
	rs.smu.Unlock()
	if !ok {
		return fmt.Errorf("no active session for server %s", serverID)
	}
	_, err := sess.Stdin().Write(data)
	return err
}

// Subscribe returns a channel of Events for the run, plus an unsubscribe
// function. It errors when runID is not the current run.
func (s *Service) Subscribe(runID string) (<-chan Event, func(), error) {
	s.mu.Lock()
	rs := s.current
	s.mu.Unlock()
	if rs == nil || rs.run.ID != runID {
		return nil, nil, fmt.Errorf("no batch run %s", runID)
	}

	ch := make(chan Event, 256)
	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()

	unsubscribe := func() {
		s.subsMu.Lock()
		delete(s.subs, ch)
		s.subsMu.Unlock()
	}
	return ch, unsubscribe, nil
}

// broadcast delivers an event to all subscribers, dropping it for slow
// clients (a lagging subscriber must not stall the run).
func (s *Service) broadcast(ev Event) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// runOne executes the script on one Server: write the temp script file, open
// the PTY session, stream merged output, then finalize the Run Result.
func (s *Service) runOne(ctx context.Context, rs *runState, result *model.RunResult) {
	defer rs.wg.Done()

	now := s.now()
	result.Status = model.ResultRunning
	result.StartedAt = &now
	_ = s.persistResult(rs, result)
	s.emitStatus(rs, result)

	if err := s.writeScript(ctx, rs.run.ID, result.ServerID, rs.run.Script); err != nil {
		s.finalizeFailed(ctx, rs, result, err.Error())
		return
	}

	sess, err := s.opener.OpenCommand(result.ServerID, "sh "+scriptPath(rs.run.ID), pty.DefaultRows, pty.DefaultCols)
	if err != nil {
		s.finalizeFailed(ctx, rs, result, err.Error())
		return
	}

	rs.smu.Lock()
	rs.sessions[result.ServerID] = sess
	rs.smu.Unlock()
	defer func() {
		rs.smu.Lock()
		delete(rs.sessions, result.ServerID)
		rs.smu.Unlock()
		sess.Close()
	}()

	// A cancel racing between session registration and the read loop must not
	// strand the session: close it here and finalize as cancelled.
	if ctx.Err() != nil {
		sess.Close()
		s.finalizeFailed(ctx, rs, result, errCancelled)
		return
	}

	// Pump merged output into the stream: broadcast chunks live and buffer
	// them in memory (truncated at the cap). Persistence is throttled to at
	// most one write per flushInterval — a burst of output must not trigger a
	// store write per chunk. The loop runs until EOF (process exit) or
	// cancel, so output keeps flowing while the process is alive.
	lastFlush := time.Time{}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, readBufSize)
		for {
			n, err := sess.Stdout().Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				s.appendOutput(result, chunk)
				s.broadcast(Event{Type: "output", ServerID: result.ServerID, Data: string(chunk)})
				if s.now().Sub(lastFlush) >= s.flushInterval {
					_ = s.persistResult(rs, result)
					lastFlush = s.now()
				}
			}
			if err != nil {
				return
			}
		}
	}()

	exitStatus, waitErr := sess.Wait()
	// The read loop drains any output still buffered in the PTY stream after
	// the process exited; wait for it so the terminal state below sees the
	// complete output.
	<-readDone

	if ctx.Err() != nil {
		s.finalizeFailed(ctx, rs, result, errCancelled)
		return
	}
	if waitErr != nil {
		s.finalizeFailed(ctx, rs, result, waitErr.Error())
		return
	}
	// The pty bridge resolves the OpenSSH exit-status unreliability: a
	// dropped exit-status message comes back as an unknown status, which with
	// the output stream intact is a success without a recorded exit code. A
	// real non-zero status is a failure.
	if exitStatus.Known && exitStatus.Code != 0 {
		result.Status = model.ResultFailed
		result.Error = fmt.Sprintf("exit status %d", exitStatus.Code)
		result.ExitCode = &exitStatus.Code
	} else {
		result.Status = model.ResultSuccess
		if exitStatus.Known && exitStatus.Code == 0 {
			code := 0
			result.ExitCode = &code
		}
	}
	now = s.now()
	result.FinishedAt = &now
	if err := s.persistResult(rs, result); err != nil {
		// The recorded outcome could not be persisted — surface the
		// persistence failure on the result instead of logging it away.
		result.Status = model.ResultFailed
		result.Error = fmt.Sprintf("persist result: %v", err)
		_ = s.persistResult(rs, result)
	}
	s.emitStatus(rs, result)
}

// writeScript writes the script to a remote temp file via base64, the same
// technique the cron writer uses, so arbitrary content needs no escaping.
func (s *Service) writeScript(ctx context.Context, runID, serverID, script string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	cmd := fmt.Sprintf("printf '%%s' %s | base64 -d > %s", encoded, scriptPath(runID))
	_, err := s.exec.Execute(ctx, serverID, cmd)
	return err
}

// scriptPath is the deterministic remote temp file for a run. The run ID is a
// UUID, so the path is shell-safe.
func scriptPath(runID string) string {
	return "/tmp/belochka-batch-" + runID + ".sh"
}

// newRun creates a fresh running Batch Run with the given script and target
// servers, generating its ID and timestamp. Results start in pending state.
func (s *Service) newRun(script string, serverIDs []string) (model.BatchRun, []model.RunResult) {
	run := model.BatchRun{
		ID:        uuid.New().String(),
		Script:    script,
		Status:    model.BatchRunRunning,
		CreatedAt: s.now(),
	}
	results := make([]model.RunResult, 0, len(serverIDs))
	for _, serverID := range serverIDs {
		results = append(results, model.RunResult{
			RunID:    run.ID,
			ServerID: serverID,
			Status:   model.ResultPending,
		})
	}
	return run, results
}

// appendOutput appends a stream chunk to the result's in-memory output,
// truncating at the cap. The truncated flag is set once output is dropped.
// Persistence is handled by the read loop's flush cadence, not per chunk.
func (s *Service) appendOutput(result *model.RunResult, chunk []byte) {
	if result.Truncated {
		return
	}
	room := maxOutputBytes - len(result.Output)
	if len(chunk) > room {
		result.Output += string(chunk[:room])
		result.Truncated = true
	} else {
		result.Output += string(chunk)
	}
}

// finalizeFailed ends a result as failed. A cancel takes precedence so every
// unfinished result ends with the canonical "cancelled" error.
func (s *Service) finalizeFailed(ctx context.Context, rs *runState, result *model.RunResult, errMsg string) {
	if ctx.Err() != nil {
		errMsg = errCancelled
	}
	result.Status = model.ResultFailed
	result.Error = errMsg
	now := s.now()
	result.FinishedAt = &now
	_ = s.persistResult(rs, result)
	s.emitStatus(rs, result)
}

// persistResult writes the result's current in-memory state to the store.
// The error is returned for callers that need to surface a persistence
// failure; transient mid-run failures are retried by the flush cadence.
func (s *Service) persistResult(rs *runState, result *model.RunResult) error {
	if err := s.store.UpdateBatchRunResult(context.Background(), *result); err != nil {
		slog.Error("batchrun: persist result", "run_id", rs.run.ID, "server_id", result.ServerID, "error", err)
		return err
	}
	return nil
}

// emitStatus broadcasts a status transition for a result.
func (s *Service) emitStatus(rs *runState, result *model.RunResult) {
	s.broadcast(Event{
		Type:       "status",
		ServerID:   result.ServerID,
		Status:     result.Status,
		ExitCode:   result.ExitCode,
		Truncated:  result.Truncated,
		Error:      result.Error,
		FinishedAt: (*EventTime)(result.FinishedAt),
	})
}
