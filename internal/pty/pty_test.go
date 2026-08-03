package pty

import (
	"errors"
	"io"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// fakeRemote is a scriptable remoteSession. The Open helpers bind it
// directly, bypassing NewOpener's gossh adapter.
type fakeRemote struct {
	term          string
	rows, cols    int
	modes         gossh.TerminalModes
	started       string // "shell" or the started command
	waitCode      int
	waitErr       error
	ptyErr        error
	startErr      error
	stdoutPipeErr error
	stdinPipeErr  error
	resizes       []windowChange
	closed        bool

	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
}

type windowChange struct {
	rows, cols int
}

func newFakeRemote() *fakeRemote {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	_ = stdinR
	_ = stdoutW
	return &fakeRemote{stdinW: stdinW, stdoutR: stdoutR}
}

func (f *fakeRemote) RequestPty(term string, rows, cols int, modes gossh.TerminalModes) error {
	f.term, f.rows, f.cols, f.modes = term, rows, cols, modes
	return f.ptyErr
}

func (f *fakeRemote) WindowChange(rows, cols int) error {
	f.resizes = append(f.resizes, windowChange{rows: rows, cols: cols})
	return nil
}

func (f *fakeRemote) StdinPipe() (io.WriteCloser, error) {
	return f.stdinW, f.stdinPipeErr
}

func (f *fakeRemote) StdoutPipe() (io.Reader, error) {
	return f.stdoutR, f.stdoutPipeErr
}

func (f *fakeRemote) Start(cmd string) error {
	f.started = cmd
	return f.startErr
}

func (f *fakeRemote) Shell() error {
	f.started = "shell"
	return f.startErr
}

func (f *fakeRemote) Wait() (int, error) {
	return f.waitCode, f.waitErr
}

func (f *fakeRemote) Close() error {
	f.closed = true
	return nil
}

func openerWith(fake *fakeRemote) *Opener {
	return &Opener{open: func(string) (remoteSession, error) { return fake, nil }}
}

func TestOpenInteractiveRequestsPTYAndStartsShell(t *testing.T) {
	fake := newFakeRemote()
	sess, err := openerWith(fake).OpenInteractive("srv1", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if fake.term != "xterm-256color" || fake.rows != 24 || fake.cols != 80 {
		t.Fatalf("unexpected PTY request: term=%q %dx%d", fake.term, fake.rows, fake.cols)
	}
	if fake.started != "shell" {
		t.Fatalf("expected shell start, got %q", fake.started)
	}
	if fake.closed {
		t.Fatal("session closed on success")
	}
	if sess.Stdin() != fake.stdinW || sess.Stdout() != fake.stdoutR {
		t.Fatal("session pipes not wired from the remote session")
	}
}

func TestOpenCommandStartsCommand(t *testing.T) {
	fake := newFakeRemote()
	if _, err := openerWith(fake).OpenCommand("srv1", "sh /tmp/x.sh", 24, 80); err != nil {
		t.Fatalf("open: %v", err)
	}
	if fake.started != "sh /tmp/x.sh" {
		t.Fatalf("expected the command to start, got %q", fake.started)
	}
}

func TestOpenFailureWrappedInErrOpenSession(t *testing.T) {
	o := &Opener{open: func(string) (remoteSession, error) {
		return nil, errors.New("no connection for server X")
	}}
	_, err := o.OpenInteractive("srv1", 24, 80)
	if !errors.Is(err, ErrOpenSession) {
		t.Fatalf("expected ErrOpenSession, got %v", err)
	}
}

func TestPTYFailureClosesSession(t *testing.T) {
	fake := newFakeRemote()
	fake.ptyErr = errors.New("pty refused")
	if _, err := openerWith(fake).OpenInteractive("srv1", 24, 80); err == nil {
		t.Fatal("expected PTY error")
	}
	if !fake.closed {
		t.Fatal("session not closed after PTY failure")
	}
}

func TestPipeFailureClosesSession(t *testing.T) {
	fake := newFakeRemote()
	fake.stdoutPipeErr = errors.New("pipe failed")
	if _, err := openerWith(fake).OpenInteractive("srv1", 24, 80); err == nil {
		t.Fatal("expected pipe error")
	}
	if !fake.closed {
		t.Fatal("session not closed after pipe failure")
	}
}

func TestStartFailureClosesSession(t *testing.T) {
	fake := newFakeRemote()
	fake.startErr = errors.New("start failed")
	if _, err := openerWith(fake).OpenInteractive("srv1", 24, 80); err == nil {
		t.Fatal("expected start error")
	}
	if !fake.closed {
		t.Fatal("session not closed after start failure")
	}
}

// TestWaitUnknownExitStatusIsKnownFalse asserts that the "unknown" status a
// PTY reports when OpenSSH drops the exit-status message (gossh -1) comes
// back as Known: false, never as a real exit code.
func TestWaitUnknownExitStatusIsKnownFalse(t *testing.T) {
	fake := newFakeRemote()
	fake.waitCode = -1
	sess, err := openerWith(fake).OpenCommand("srv1", "sh /tmp/x.sh", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	status, err := sess.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if status.Known || status.Code != 0 {
		t.Fatalf("expected unknown status, got %+v", status)
	}
}

func TestWaitCleanExitIsKnownZero(t *testing.T) {
	fake := newFakeRemote()
	fake.waitCode = 0
	sess, err := openerWith(fake).OpenCommand("srv1", "sh /tmp/x.sh", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	status, err := sess.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !status.Known || status.Code != 0 {
		t.Fatalf("expected known zero status, got %+v", status)
	}
}

func TestWaitNonZeroExitCarriesCode(t *testing.T) {
	fake := newFakeRemote()
	fake.waitCode = 3
	sess, err := openerWith(fake).OpenCommand("srv1", "sh /tmp/x.sh", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	status, err := sess.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !status.Known || status.Code != 3 {
		t.Fatalf("expected known status 3, got %+v", status)
	}
}

func TestWaitSessionErrorPropagates(t *testing.T) {
	fake := newFakeRemote()
	fake.waitErr = errors.New("session terminated")
	sess, err := openerWith(fake).OpenCommand("srv1", "sh /tmp/x.sh", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	status, err := sess.Wait()
	if err == nil {
		t.Fatal("expected wait error")
	}
	if status != (ExitStatus{}) {
		t.Fatalf("expected zero status on error, got %+v", status)
	}
}

func TestSetSizeForwardsWindowChange(t *testing.T) {
	fake := newFakeRemote()
	sess, err := openerWith(fake).OpenInteractive("srv1", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := sess.SetSize(40, 120); err != nil {
		t.Fatalf("set size: %v", err)
	}
	if len(fake.resizes) != 1 || fake.resizes[0] != (windowChange{rows: 40, cols: 120}) {
		t.Fatalf("expected one 40x120 resize, got %v", fake.resizes)
	}
}

func TestCloseClosesRemote(t *testing.T) {
	fake := newFakeRemote()
	sess, err := openerWith(fake).OpenInteractive("srv1", 24, 80)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !fake.closed {
		t.Fatal("remote session not closed")
	}
}
