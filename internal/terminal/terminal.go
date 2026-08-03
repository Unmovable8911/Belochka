package terminal

import (
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"sync"

	"belochka/internal/pty"
	"belochka/internal/wsutil"
	"github.com/gorilla/websocket"
)

const readBufferSize = 4096

// PTYOpener opens interactive PTY sessions on remote Servers. Satisfied by
// *pty.Opener.
type PTYOpener interface {
	OpenInteractive(serverID string, rows, cols int) (pty.Session, error)
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
	opener   PTYOpener
	mu       sync.Mutex
	sessions map[pty.Session]struct{}
}

// NewHandler creates a new terminal Handler.
func NewHandler(opener PTYOpener) *Handler {
	return &Handler{
		opener:   opener,
		sessions: make(map[pty.Session]struct{}),
	}
}

// CloseAll closes all active terminal sessions.
func (h *Handler) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.sessions {
		s.Close()
	}
	h.sessions = make(map[pty.Session]struct{})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverID := path.Base(r.URL.Path)
	if serverID == "" || serverID == "." {
		http.Error(w, "missing server ID", http.StatusBadRequest)
		return
	}

	session, err := h.opener.OpenInteractive(serverID, pty.DefaultRows, pty.DefaultCols)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, pty.ErrOpenSession) {
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

	go h.bridgeSession(conn, session)
}

func sendStatus(conn *websocket.Conn, status, message string) {
	msg := statusMessage{Type: "status", Status: status, Message: message}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

func (h *Handler) bridgeSession(conn *websocket.Conn, session pty.Session) {
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
			n, err := session.Stdout().Read(buf)
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
				session.Stdin().Close()
				session.Close()
				return
			}
			switch msgType {
			case websocket.BinaryMessage:
				session.Stdin().Write(data)
			case websocket.TextMessage:
				var ctrl struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" {
					session.SetSize(ctrl.Rows, ctrl.Cols)
				}
			}
		}
	}()

	<-done
	sendStatus(conn, "disconnected", "session ended")
	<-wsDone
}
