package batchrun_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"belochka/internal/batchrun"
	"belochka/internal/model"
	"belochka/internal/pty"
)

// --- Fakes ---

// fakeStore records the current run and per-server results, mirroring the
// serialized single-run persistence contract.
type fakeStore struct {
	mu          sync.Mutex
	run         model.BatchRun
	exists      bool
	results     []model.RunResult
	runDone     int // number of SetBatchRunDone calls
	replaced    []model.BatchRun
	updateCalls int // number of UpdateBatchRunResult calls
	updateErr   error
}

func (f *fakeStore) ReplaceBatchRun(_ context.Context, run model.BatchRun, results []model.RunResult) (model.BatchRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.exists && f.run.Status == model.BatchRunRunning {
		return model.BatchRun{}, model.ErrBatchRunInProgress
	}
	f.run = run
	f.exists = true
	f.results = append([]model.RunResult(nil), results...)
	f.replaced = append(f.replaced, run)
	return run, nil
}

func (f *fakeStore) GetBatchRun(_ context.Context) (model.BatchRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.exists {
		return model.BatchRun{}, model.ErrBatchRunNotFound
	}
	return f.run, nil
}

func (f *fakeStore) ListBatchRunResults(_ context.Context, runID string) ([]model.RunResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.RunResult
	for _, r := range f.results {
		if r.RunID == runID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) UpdateBatchRunResult(_ context.Context, r model.RunResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls++
	if f.updateErr != nil {
		return f.updateErr
	}
	for i := range f.results {
		if f.results[i].ServerID == r.ServerID {
			f.results[i] = r
			return nil
		}
	}
	f.results = append(f.results, r)
	return nil
}

func (f *fakeStore) SetBatchRunDone(_ context.Context, runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.run.ID == runID {
		f.run.Status = model.BatchRunDone
		f.runDone++
	}
	return nil
}

func (f *fakeStore) result(serverID string) model.RunResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.results {
		if r.ServerID == serverID {
			return r
		}
	}
	return model.RunResult{}
}

func (f *fakeStore) status() model.BatchRunStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.exists {
		return ""
	}
	return f.run.Status
}

// fakeExecutor fails per-server on demand and records every command.
type fakeExecutor struct {
	mu     sync.Mutex
	errs   map[string]error
	cmds   []string
}

func (f *fakeExecutor) Execute(_ context.Context, serverID, cmd string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds = append(f.cmds, cmd)
	if err := f.errs[serverID]; err != nil {
		return "", err
	}
	return "", nil
}

func (f *fakeExecutor) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cmds...)
}

// fakeSession is a pty.Session whose lifecycle the test controls: Wait
// returns when waitReturn is closed (normal exit) or when Close is called
// (cancel).
type fakeSession struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter

	mu         sync.Mutex
	closed     bool
	closedCh   chan struct{}
	waitReturn chan struct{}
	status     pty.ExitStatus
	waitErr    error
}

func newFakeSession() *fakeSession {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	return &fakeSession{
		stdinR:     stdinR,
		stdinW:     stdinW,
		stdoutR:    stdoutR,
		stdoutW:    stdoutW,
		closedCh:   make(chan struct{}),
		waitReturn: make(chan struct{}),
	}
}

func (s *fakeSession) Stdin() io.WriteCloser { return s.stdinW }

func (s *fakeSession) Stdout() io.Reader { return s.stdoutR }

func (s *fakeSession) SetSize(_, _ int) error { return nil }

func (s *fakeSession) Wait() (pty.ExitStatus, error) {
	select {
	case <-s.waitReturn:
		return s.status, s.waitErr
	case <-s.closedCh:
		return pty.ExitStatus{}, errors.New("session closed")
	}
}

func (s *fakeSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.closedCh)
	s.mu.Unlock()
	s.stdinW.Close()
	s.stdoutW.Close()
	return nil
}

func (s *fakeSession) finish(status pty.ExitStatus, err error) {
	s.mu.Lock()
	s.status = status
	s.waitErr = err
	s.mu.Unlock()
	close(s.waitReturn)
	// A process exit delivers EOF on the PTY output stream.
	s.stdoutW.Close()
}

func (s *fakeSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// fakeOpener opens sessions per server, failing on demand.
type fakeOpener struct {
	mu       sync.Mutex
	sessions map[string]*fakeSession
	openErr  map[string]error
	opens    []string // serverIDs in open order
}

func newFakeOpener() *fakeOpener {
	return &fakeOpener{sessions: make(map[string]*fakeSession), openErr: make(map[string]error)}
}

func (r *fakeOpener) OpenCommand(serverID, _ string, _, _ int) (pty.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opens = append(r.opens, serverID)
	if err := r.openErr[serverID]; err != nil {
		return nil, err
	}
	s := newFakeSession()
	r.sessions[serverID] = s
	return s, nil
}

func (r *fakeOpener) session(serverID string) *fakeSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sessions[serverID]
}

func (r *fakeOpener) openedServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.opens...)
}

// --- Helpers ---

func waitFor(t *testing.T, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", msg)
}

type testEnv struct {
	store  *fakeStore
	exec   *fakeExecutor
	opener *fakeOpener
	svc    *batchrun.Service
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	env := &testEnv{
		store:  &fakeStore{},
		exec:   &fakeExecutor{errs: make(map[string]error)},
		opener: newFakeOpener(),
	}
	env.svc = batchrun.New(env.store, env.exec, env.opener)
	t.Cleanup(env.svc.Cancel)
	return env
}

// --- Tests ---

func TestDispatchRunsScriptOnEveryServer(t *testing.T) {
	env := newEnv(t)

	run, results, err := env.svc.Dispatch(context.Background(), "echo hi", []string{"s1", "s2", "s3"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if run.Status != model.BatchRunRunning {
		t.Fatalf("expected running run, got %s", run.Status)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != model.ResultPending {
			t.Fatalf("expected pending result for %s, got %s", r.ServerID, r.Status)
		}
	}

	// Every target server must be started.
	waitFor(t, 2*time.Second, func() bool {
		return len(env.opener.openedServers()) == 3
	}, "all three servers started")

	// The temp script file write must use the base64 technique.
	cmds := env.exec.commands()
	if len(cmds) != 3 {
		t.Fatalf("expected 3 script writes, got %d", len(cmds))
	}
	for _, cmd := range cmds {
		if !strings.Contains(cmd, "printf '%s' ") || !strings.Contains(cmd, "base64 -d > /tmp/belochka-batch-"+run.ID+".sh") {
			t.Fatalf("unexpected script write command: %s", cmd)
		}
	}

	// Finish all sessions successfully.
	for _, id := range []string{"s1", "s2", "s3"} {
		env.opener.session(id).finish(pty.ExitStatus{Known: true}, nil)
	}

	waitFor(t, 2*time.Second, func() bool {
		for _, id := range []string{"s1", "s2", "s3"} {
			if r := env.store.result(id); r.Status != model.ResultSuccess {
				return false
			}
		}
		return true
	}, "all results succeed")
	waitFor(t, 2*time.Second, func() bool { return env.store.status() == model.BatchRunDone }, "run marked done")

	for _, id := range []string{"s1", "s2", "s3"} {
		r := env.store.result(id)
		if r.ExitCode == nil || *r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 for %s, got %v", id, r.ExitCode)
		}
		if r.StartedAt == nil || r.FinishedAt == nil {
			t.Fatalf("expected timestamps on %s result", id)
		}
	}
}

func TestNonZeroExitFailsResult(t *testing.T) {
	env := newEnv(t)

	_, _, err := env.svc.Dispatch(context.Background(), "exit 3", []string{"s1"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool { return env.opener.session("s1") != nil }, "session opened")
	env.opener.session("s1").finish(pty.ExitStatus{Known: true, Code: 3}, nil)

	waitFor(t, 2*time.Second, func() bool {
		r := env.store.result("s1")
		return r.Status == model.ResultFailed && r.Error == "exit status 3" &&
			r.ExitCode != nil && *r.ExitCode == 3
	}, "result failed with exit status 3")
}

// TestUnknownExitStatusSucceeds asserts that the "unknown" exit status a PTY
// reports when OpenSSH drops the exit-status message — resolved by the pty
// bridge to ExitStatus{Known: false} — is treated as a success without a
// recorded exit code, not as a failure.
func TestUnknownExitStatusSucceeds(t *testing.T) {
	env := newEnv(t)

	_, _, err := env.svc.Dispatch(context.Background(), "echo $PATH", []string{"s1"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool { return env.opener.session("s1") != nil }, "session opened")
	env.opener.session("s1").finish(pty.ExitStatus{}, nil)

	waitFor(t, 2*time.Second, func() bool {
		r := env.store.result("s1")
		return r.Status == model.ResultSuccess && r.ExitCode == nil
	}, "result succeeded without an exit code")
}

func TestUnreachableServerFailsImmediately(t *testing.T) {
	env := newEnv(t)
	env.exec.errs["s1"] = errors.New("server s1 not connected")

	_, _, err := env.svc.Dispatch(context.Background(), "echo hi", []string{"s1"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// The connection failure must fail the result without ever opening a
	// session, and must not block the other servers in the batch.
	waitFor(t, 2*time.Second, func() bool {
		r := env.store.result("s1")
		return r.Status == model.ResultFailed && strings.Contains(r.Error, "not connected")
	}, "result failed with connection error")

	if len(env.opener.openedServers()) != 0 {
		t.Fatalf("session opened for unreachable server")
	}
}

func TestDispatchWhileRunningReturnsConflict(t *testing.T) {
	env := newEnv(t)

	if _, _, err := env.svc.Dispatch(context.Background(), "echo one", []string{"s1"}); err != nil {
		t.Fatalf("first dispatch: %v", err)
	}

	_, _, err := env.svc.Dispatch(context.Background(), "echo two", []string{"s2"})
	if !errors.Is(err, model.ErrBatchRunInProgress) {
		t.Fatalf("expected ErrBatchRunInProgress, got %v", err)
	}
}

func TestCancelClosesSessionsAndFailsRunningResults(t *testing.T) {
	env := newEnv(t)

	if _, _, err := env.svc.Dispatch(context.Background(), "sleep 999", []string{"s1", "s2"}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		return env.opener.session("s1") != nil && env.opener.session("s2") != nil
	}, "both sessions opened")

	env.svc.Cancel()

	waitFor(t, 2*time.Second, func() bool {
		return env.opener.session("s1").isClosed() && env.opener.session("s2").isClosed()
	}, "sessions closed on cancel")
	waitFor(t, 2*time.Second, func() bool {
		for _, id := range []string{"s1", "s2"} {
			r := env.store.result(id)
			if r.Status != model.ResultFailed || r.Error != "cancelled" {
				return false
			}
		}
		return true
	}, "results failed with cancelled")
	waitFor(t, 2*time.Second, func() bool { return env.store.status() == model.BatchRunDone }, "run marked done after cancel")

	// A new dispatch is possible after cancel completes.
	waitFor(t, 2*time.Second, func() bool { return env.store.runDone > 0 }, "run done persisted")
	if _, _, err := env.svc.Dispatch(context.Background(), "echo again", []string{"s3"}); err != nil {
		t.Fatalf("dispatch after cancel: %v", err)
	}
}

func TestOutputStreamedAndTruncatedAtCap(t *testing.T) {
	env := newEnv(t)

	stop := make(chan struct{})
	defer close(stop)
	events := []batchrun.Event{}
	var mu sync.Mutex
	ch, _, err := env.svc.Subscribe(mustDispatch(t, env))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go func() {
		for {
			select {
			case ev := <-ch:
				mu.Lock()
				events = append(events, ev)
				mu.Unlock()
			case <-stop:
				return
			}
		}
	}()

	sess := env.opener.session("s1")
	// 20 × 4 KiB = 80 KiB of output, exceeding the 64 KiB cap.
	for i := 0; i < 20; i++ {
		if _, err := sess.stdoutW.Write(make([]byte, 4096)); err != nil {
			t.Fatalf("write output: %v", err)
		}
	}
	sess.finish(pty.ExitStatus{Known: true}, nil)

	waitFor(t, 2*time.Second, func() bool {
		r := env.store.result("s1")
		return r.Status == model.ResultSuccess && r.Truncated
	}, "result truncated")

	r := env.store.result("s1")
	if len(r.Output) != 64*1024 {
		t.Fatalf("expected output capped at 64 KiB, got %d bytes", len(r.Output))
	}

	// The live stream must deliver all output, not just the persisted cap.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		total := 0
		for _, ev := range events {
			if ev.Type == "output" {
				total += len(ev.Data)
			}
		}
		return total == 80*1024
	}, "all output streamed to subscribers")
}

// TestOutputStreamedLive asserts output reaches subscribers while the process
// is still running — the interactive batch promise of seeing prompts while a
// script waits for input, rather than a burst at exit.
func TestOutputStreamedLive(t *testing.T) {
	env := newEnv(t)

	stop := make(chan struct{})
	defer close(stop)
	var total int
	var mu sync.Mutex
	ch, _, err := env.svc.Subscribe(mustDispatch(t, env))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go func() {
		for {
			select {
			case ev := <-ch:
				if ev.Type == "output" {
					mu.Lock()
					total += len(ev.Data)
					mu.Unlock()
				}
			case <-stop:
				return
			}
		}
	}()

	sess := env.opener.session("s1")
	sess.stdoutW.Write([]byte("Are you sure? [y/N] "))

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return total == len("Are you sure? [y/N] ")
	}, "prompt delivered while process still running")

	// The session was never finished while the prompt arrived — finish it now
	// so the run completes cleanly.
	sess.finish(pty.ExitStatus{Known: true}, nil)
}

// TestFlushAggregatesWrites asserts a burst of output does not trigger a
// store write per chunk: the interval flush batches chunk writes, and the
// final persist carries the remaining output.
func TestFlushAggregatesWrites(t *testing.T) {
	env := newEnv(t)
	env.svc = batchrun.New(env.store, env.exec, env.opener, batchrun.WithFlushInterval(time.Second))

	_, _, err := env.svc.Dispatch(context.Background(), "yes", []string{"s1"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return env.opener.session("s1") != nil }, "session opened")

	sess := env.opener.session("s1")
	// 20 × 4 KiB = 80 KiB written in a burst, well within one flush interval.
	for i := 0; i < 20; i++ {
		if _, err := sess.stdoutW.Write(make([]byte, 4096)); err != nil {
			t.Fatalf("write output: %v", err)
		}
	}
	sess.finish(pty.ExitStatus{Known: true}, nil)

	waitFor(t, 2*time.Second, func() bool {
		r := env.store.result("s1")
		return r.Status == model.ResultSuccess && len(r.Output) == 64*1024 && r.Truncated
	}, "result persisted with capped output")

	calls := env.store.updateCalls
	if calls > 5 {
		t.Fatalf("expected aggregated writes (≤5), got %d for 20 chunks", calls)
	}
}

// TestTerminalPersistFailureFailsResult asserts a failure to persist the
// terminal state is surfaced on the result — visible to subscribers — rather
// than logged away.
func TestTerminalPersistFailureFailsResult(t *testing.T) {
	env := newEnv(t)
	env.store.updateErr = errors.New("disk full")

	stop := make(chan struct{})
	defer close(stop)
	var failed *batchrun.Event
	var mu sync.Mutex
	ch, _, err := env.svc.Subscribe(mustDispatch(t, env))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go func() {
		for {
			select {
			case ev := <-ch:
				if ev.Type == "status" && ev.Status == model.ResultFailed {
					mu.Lock()
					failed = &ev
					mu.Unlock()
				}
			case <-stop:
				return
			}
		}
	}()

	sess := env.opener.session("s1")
	sess.stdoutW.Write([]byte("hello"))
	sess.finish(pty.ExitStatus{Known: true}, nil)

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return failed != nil && strings.Contains(failed.Error, "persist result")
	}, "failed status event carries the persist error")
}

func TestNewDispatchReplacesPreviousRun(t *testing.T) {
	env := newEnv(t)

	run1, _, err := env.svc.Dispatch(context.Background(), "echo one", []string{"s1"})
	if err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return env.opener.session("s1") != nil }, "first session opened")
	env.opener.session("s1").finish(pty.ExitStatus{Known: true}, nil)
	waitFor(t, 2*time.Second, func() bool { return env.store.status() == model.BatchRunDone }, "first run done")

	run2, results2, err := env.svc.Dispatch(context.Background(), "echo two", []string{"s2", "s3"})
	if err != nil {
		t.Fatalf("second dispatch: %v", err)
	}

	// The previous run's records are gone: current only knows run2.
	cur, curResults, err := env.svc.Current(context.Background())
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if cur.ID != run2.ID || cur.ID == run1.ID {
		t.Fatalf("expected current run %s, got %s", run2.ID, cur.ID)
	}
	if len(curResults) != 2 {
		t.Fatalf("expected 2 current results, got %d", len(curResults))
	}
	for i, r := range curResults {
		if r.ServerID != results2[i].ServerID {
			t.Fatalf("result order mismatch: got %s want %s", r.ServerID, results2[i].ServerID)
		}
	}
}

func TestInputWritesToSessionAndRejectsAfterExit(t *testing.T) {
	env := newEnv(t)

	run, _, err := env.svc.Dispatch(context.Background(), "read x", []string{"s1"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return env.opener.session("s1") != nil }, "session opened")

	// Verify the data reached the session's stdin.
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 8)
		n, _ := env.opener.session("s1").stdinR.Read(buf)
		got <- string(buf[:n])
	}()
	if err := env.svc.Input(run.ID, "s1", []byte("y\n")); err != nil {
		t.Fatalf("input while running: %v", err)
	}
	select {
	case s := <-got:
		if s != "y\n" {
			t.Fatalf("expected 'y\\n' on stdin, got %q", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout reading session stdin")
	}

	env.opener.session("s1").finish(pty.ExitStatus{Known: true}, nil)
	waitFor(t, 2*time.Second, func() bool {
		return env.store.result("s1").Status == model.ResultSuccess
	}, "result success")

	if err := env.svc.Input(run.ID, "s1", []byte("z\n")); err == nil {
		t.Fatal("expected input to be rejected after the process exited")
	}
}

func TestSubscribeRejectsUnknownRun(t *testing.T) {
	env := newEnv(t)
	if _, _, err := env.svc.Subscribe("no-such-run"); err == nil {
		t.Fatal("expected error for unknown run")
	}
}

func TestSubscriberStreamClosedOnRunCompletion(t *testing.T) {
	env := newEnv(t)

	ch, _, err := env.svc.Subscribe(mustDispatch(t, env))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	env.opener.session("s1").finish(pty.ExitStatus{Known: true}, nil)

	// The stream closes once the run completes; the WS handler turns that
	// into a connection close, the client's "run finished" signal.
	waitFor(t, 2*time.Second, func() bool {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return true
				}
			default:
				return false
			}
		}
	}, "subscriber channel closed on run completion")
}

func TestCancelAlsoClosesSubscriberStreams(t *testing.T) {
	env := newEnv(t)

	ch, _, err := env.svc.Subscribe(mustDispatch(t, env))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	env.svc.Cancel()

	waitFor(t, 2*time.Second, func() bool {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return true
				}
			default:
				return false
			}
		}
	}, "subscriber channel closed after cancel")
}

// TestStatusEventsCarryExitCodeAndError asserts the terminal status event
// payload (exit code and error). The "running" transition is emitted before a
// subscriber can attach, so only events after subscription are asserted —
// consistent with the protocol, where the dialog fetches initial state via
// REST before subscribing.
func TestStatusEventsCarryExitCodeAndError(t *testing.T) {
	env := newEnv(t)

	stop := make(chan struct{})
	defer close(stop)
	var mu sync.Mutex
	var events []batchrun.Event
	runID := mustDispatch(t, env)
	ch, _, err := env.svc.Subscribe(runID)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	go func() {
		for {
			select {
			case ev := <-ch:
				mu.Lock()
				events = append(events, ev)
				mu.Unlock()
			case <-stop:
				return
			}
		}
	}()

	sess := env.opener.session("s1")
	sess.stdoutW.Write([]byte("hello"))
	sess.finish(pty.ExitStatus{Known: true, Code: 3}, nil)

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) >= 2
	}, "output and failed status events")

	mu.Lock()
	defer mu.Unlock()
	var failed *batchrun.Event
	output := ""
	for i := range events {
		ev := events[i]
		if ev.Type == "status" && ev.Status == model.ResultFailed {
			failed = &events[i]
		} else if ev.Type == "output" {
			output += ev.Data
		}
	}
	if failed == nil {
		t.Fatalf("expected a failed status event, got %d events", len(events))
	}
	if failed.ExitCode == nil || *failed.ExitCode != 3 {
		t.Fatalf("expected exit code 3 on failed event, got %v", failed.ExitCode)
	}
	if failed.Error != "exit status 3" {
		t.Fatalf("expected 'exit status 3' error, got %q", failed.Error)
	}
	if output != "hello" {
		t.Fatalf("expected 'hello' output event, got %q", output)
	}
}

func TestStaleRunningRunFinalizedOnNew(t *testing.T) {
	store := &fakeStore{}
	// Seed a run left running by a previous process.
	store.exists = true
	store.run = model.BatchRun{ID: "stale-1", Script: "old", Status: model.BatchRunRunning, CreatedAt: time.Now()}
	now := time.Now()
	store.results = []model.RunResult{
		{RunID: "stale-1", ServerID: "s1", Status: model.ResultRunning, StartedAt: &now},
		{RunID: "stale-1", ServerID: "s2", Status: model.ResultSuccess},
	}

	svc := batchrun.New(store, &fakeExecutor{errs: map[string]error{}}, newFakeOpener())
	defer svc.Cancel()

	waitFor(t, 2*time.Second, func() bool { return store.status() == model.BatchRunDone }, "stale run marked done")

	if r := store.result("s1"); r.Status != model.ResultFailed || r.Error != "interrupted by restart" {
		t.Fatalf("expected running result failed as interrupted, got %s / %q", r.Status, r.Error)
	}
	if r := store.result("s2"); r.Status != model.ResultSuccess {
		t.Fatalf("expected finished result untouched, got %s", r.Status)
	}

	// A new dispatch is not blocked by the stale run.
	if _, _, err := svc.Dispatch(context.Background(), "echo fresh", []string{"s3"}); err != nil {
		t.Fatalf("dispatch after stale finalize: %v", err)
	}
}

// mustDispatch dispatches a run and returns its run ID, failing the test on
// error. Convenience for tests that subscribe before touching sessions.
func mustDispatch(t *testing.T, env *testEnv) string {
	t.Helper()
	run, _, err := env.svc.Dispatch(context.Background(), "echo hi", []string{"s1"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return env.opener.session("s1") != nil }, "session opened")
	return run.ID
}
