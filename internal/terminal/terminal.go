package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sync"

	"belochka/internal/wsutil"
	"github.com/gorilla/websocket"
)

const (
	defaultTermRows = 24
	defaultTermCols = 80
	readBufferSize  = 4096
)

// errOpenSession is a sentinel wrapped by prepareSession when OpenSession fails,
// allowing ServeHTTP to map it back to HTTP 502 Bad Gateway.
var errOpenSession = errors.New("open session")

// Session abstracts an SSH session for testability.
type Session interface {
	RequestPTY(term string, rows, cols int) error
	WindowChange(rows, cols int) error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	Shell() error
	Wait() error
	Close() error
}

// SessionOpener creates an SSH session for a given server.
type SessionOpener interface {
	OpenSession(serverID string) (Session, error)
}

// ServerNotFoundError is returned when the server ID is not in the pool.
type ServerNotFoundError struct {
	ServerID string
}

func (e *ServerNotFoundError) Error() string {
	return fmt.Sprintf("server not found: %s", e.ServerID)
}

type statusMessage struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: wsutil.CheckOrigin,
}

// Handler handles terminal WebSocket connections.
type Handler struct {
	opener   SessionOpener
	mu       sync.Mutex
	sessions map[Session]struct{}
}

// NewHandler creates a new terminal Handler.
func NewHandler(opener SessionOpener) *Handler {
	return &Handler{
		opener:   opener,
		sessions: make(map[Session]struct{}),
	}
}

// CloseAll closes all active terminal sessions.
func (h *Handler) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.sessions {
		s.Close()
	}
	h.sessions = make(map[Session]struct{})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverID := path.Base(r.URL.Path)
	if serverID == "" || serverID == "." {
		http.Error(w, "missing server ID", http.StatusBadRequest)
		return
	}

	session, stdin, stdout, err := h.prepareSession(serverID)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, errOpenSession) {
			code = http.StatusBadGateway
		}
		http.Error(w, err.Error(), code)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		session.Close()
		return
	}

	h.mu.Lock()
	h.sessions[session] = struct{}{}
	h.mu.Unlock()

	sendStatus(conn, "connected", "")

	go h.bridgeSession(conn, session, stdin, stdout)
}

// prepareSession opens an SSH session, requests a PTY, and returns the session
// along with its stdin/stdout pipes. The caller is responsible for closing the
// session on error.
func (h *Handler) prepareSession(serverID string) (Session, io.WriteCloser, io.Reader, error) {
	session, err := h.opener.OpenSession(serverID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", errOpenSession, err)
	}

	if err := session.RequestPTY("xterm-256color", defaultTermRows, defaultTermCols); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("request PTY: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("start shell: %w", err)
	}

	return session, stdin, stdout, nil
}

func sendStatus(conn *websocket.Conn, status, message string) {
	msg := statusMessage{Type: "status", Status: status, Message: message}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

func (h *Handler) bridgeSession(conn *websocket.Conn, session Session, stdin io.WriteCloser, stdout io.Reader) {
	defer func() {
		session.Close()
		h.mu.Lock()
		delete(h.sessions, session)
		h.mu.Unlock()
	}()
	defer conn.Close()

	done := make(chan struct{})

	// SSH stdout → WebSocket (binary frames)
	go func() {
		defer close(done)
		buf := make([]byte, readBufferSize)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket → SSH stdin
	wsDone := make(chan struct{})
	go func() {
		defer close(wsDone)
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				stdin.Close()
				session.Close()
				return
			}
			switch msgType {
			case websocket.BinaryMessage:
				stdin.Write(data)
			case websocket.TextMessage:
				var ctrl struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" {
					session.WindowChange(ctrl.Rows, ctrl.Cols)
				}
			}
		}
	}()

	<-done
	sendStatus(conn, "disconnected", "session ended")
	<-wsDone
}
