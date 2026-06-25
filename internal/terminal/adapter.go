package terminal

import (
	"io"

	gossh "golang.org/x/crypto/ssh"
)

// SSHSessionOpener adapts an SSH pool to the SessionOpener interface.
type SSHSessionOpener struct {
	OpenFn func(serverID string) (*gossh.Session, error)
}

func (a *SSHSessionOpener) OpenSession(serverID string) (Session, error) {
	sess, err := a.OpenFn(serverID)
	if err != nil {
		return nil, err
	}
	return &sshSession{sess: sess}, nil
}

type sshSession struct {
	sess *gossh.Session
}

func (s *sshSession) RequestPTY(term string, rows, cols int) error {
	return s.sess.RequestPty(term, rows, cols, gossh.TerminalModes{})
}

func (s *sshSession) WindowChange(rows, cols int) error {
	return s.sess.WindowChange(rows, cols)
}

func (s *sshSession) StdinPipe() (io.WriteCloser, error) {
	return s.sess.StdinPipe()
}

func (s *sshSession) StdoutPipe() (io.Reader, error) {
	return s.sess.StdoutPipe()
}

func (s *sshSession) Shell() error {
	return s.sess.Shell()
}

func (s *sshSession) Wait() error {
	return s.sess.Wait()
}

func (s *sshSession) Close() error {
	return s.sess.Close()
}
