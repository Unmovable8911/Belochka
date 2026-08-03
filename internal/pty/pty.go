// Package pty opens SSH sessions with a pseudo-terminal: it requests the
// PTY, wires the stdin/stdout pipes, starts a remote process — an
// interactive shell or a single command — and reports its exit status. Both
// the terminal console and batch command execution share this single PTY
// seam.
package pty

import (
	"errors"
	"fmt"
	"io"

	gossh "golang.org/x/crypto/ssh"
)

// Default PTY dimensions for consumers without a client-provided size.
const (
	DefaultRows = 24
	DefaultCols = 80
)

// ErrOpenSession marks a failure to open the underlying SSH session, as
// opposed to a failure after it opened (e.g. the PTY request). Callers use
// it to distinguish a broken connection from a session-level error.
var ErrOpenSession = errors.New("open session")

// ExitStatus is the outcome of a PTY process. The exit-status message a PTY
// sends is unreliable: OpenSSH frequently drops it, in which case gossh
// reports ExitStatus() == -1 ("unknown") for what was actually a clean exit.
// Wait resolves that case to Known == false — "no exit status known" — so
// callers never interpret a raw -1.
type ExitStatus struct {
	Known bool // false = the server dropped the exit-status message
	Code  int  // only meaningful when Known
}

// Session is a running PTY session on a remote Server. A PTY exposes a single
// merged output stream — stdout and stderr are not separable — and leaves
// stdin free for interactive input.
type Session interface {
	// Stdin returns the writer for interactive input.
	Stdin() io.WriteCloser
	// Stdout returns the merged terminal stream. Reads block until the
	// process exits and the stream is exhausted (or the session is closed).
	Stdout() io.Reader
	// SetSize updates the PTY dimensions.
	SetSize(rows, cols int) error
	// Wait blocks until the process exits and returns its exit status.
	Wait() (ExitStatus, error)
	// Close terminates the session, aborting the remote process.
	Close() error
}

// Opener opens PTY sessions on remote Servers: an interactive shell for the
// terminal console, or a single command for batch scripts.
type Opener struct {
	open func(serverID string) (remoteSession, error)
}

// NewOpener returns an Opener backed by a session opener. *ssh.Pool's
// OpenSession satisfies the function signature.
func NewOpener(open func(serverID string) (*gossh.Session, error)) *Opener {
	return &Opener{
		open: func(serverID string) (remoteSession, error) {
			gsess, err := open(serverID)
			if err != nil {
				return nil, err
			}
			return &gosshSession{sess: gsess}, nil
		},
	}
}

// OpenInteractive opens a PTY session running the user's default shell.
func (o *Opener) OpenInteractive(serverID string, rows, cols int) (Session, error) {
	return o.openSession(serverID, rows, cols, func(s remoteSession) error { return s.Shell() })
}

// OpenCommand opens a PTY session running the given command.
func (o *Opener) OpenCommand(serverID, command string, rows, cols int) (Session, error) {
	return o.openSession(serverID, rows, cols, func(s remoteSession) error { return s.Start(command) })
}

// openSession opens the SSH session, requests a PTY, wires the pipes, and
// starts the remote process. On any failure the underlying session is closed
// before returning.
func (o *Opener) openSession(serverID string, rows, cols int, start func(remoteSession) error) (Session, error) {
	sess, err := o.open(serverID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpenSession, err)
	}

	if err := sess.RequestPty("xterm-256color", rows, cols, gossh.TerminalModes{}); err != nil {
		sess.Close()
		return nil, fmt.Errorf("request PTY: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	if err := start(sess); err != nil {
		sess.Close()
		return nil, fmt.Errorf("start: %w", err)
	}

	return &ptySession{sess: sess, stdin: stdin, stdout: stdout}, nil
}

// remoteSession is the slice of a gossh session the bridge needs. Wait
// returns the raw exit code with the ExitError unwrapped, so the -1
// ("unknown") normalisation in ptySession.Wait is unit-testable against a
// scriptable fake instead of a real SSH connection.
type remoteSession interface {
	RequestPty(term string, rows, cols int, modes gossh.TerminalModes) error
	WindowChange(rows, cols int) error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	Start(cmd string) error
	Shell() error
	Wait() (int, error)
	Close() error
}

// gosshSession adapts a gossh session to remoteSession. Wait unwraps the
// ExitError: the raw exit code (possibly -1 when OpenSSH dropped the
// exit-status message) is reported without the error; anything else is a
// session-level failure.
type gosshSession struct {
	sess *gossh.Session
}

func (s *gosshSession) RequestPty(term string, rows, cols int, modes gossh.TerminalModes) error {
	return s.sess.RequestPty(term, rows, cols, modes)
}

func (s *gosshSession) WindowChange(rows, cols int) error {
	return s.sess.WindowChange(rows, cols)
}

func (s *gosshSession) StdinPipe() (io.WriteCloser, error) {
	return s.sess.StdinPipe()
}

func (s *gosshSession) StdoutPipe() (io.Reader, error) {
	return s.sess.StdoutPipe()
}

func (s *gosshSession) Start(cmd string) error {
	return s.sess.Start(cmd)
}

func (s *gosshSession) Shell() error {
	return s.sess.Shell()
}

func (s *gosshSession) Wait() (int, error) {
	err := s.sess.Wait()
	var exitErr *gossh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), nil
	}
	return -1, err
}

func (s *gosshSession) Close() error {
	return s.sess.Close()
}

// ptySession adapts a remote session to the Session interface.
type ptySession struct {
	sess   remoteSession
	stdin  io.WriteCloser
	stdout io.Reader
}

func (s *ptySession) Stdin() io.WriteCloser {
	return s.stdin
}

func (s *ptySession) Stdout() io.Reader {
	return s.stdout
}

func (s *ptySession) SetSize(rows, cols int) error {
	return s.sess.WindowChange(rows, cols)
}

// Wait blocks until the process exits. The exit-status message a PTY sends
// is unreliable: OpenSSH frequently drops it, and gossh then reports -1
// ("unknown") for what was actually a clean exit. That resolves to
// ExitStatus{Known: false} — callers treat it as "no exit status known",
// never as a real exit code.
func (s *ptySession) Wait() (ExitStatus, error) {
	code, err := s.sess.Wait()
	if err != nil {
		return ExitStatus{}, err
	}
	if code == -1 {
		return ExitStatus{}, nil
	}
	return ExitStatus{Known: true, Code: code}, nil
}

func (s *ptySession) Close() error {
	return s.sess.Close()
}
