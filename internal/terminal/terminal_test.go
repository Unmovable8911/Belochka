package terminal_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"belochka/internal/terminal"

	"github.com/gorilla/websocket"
)

// mockSession implements a fake SSH session using in-memory pipes.
type mockSession struct {
	Stdin       io.ReadCloser
	stdinWriter io.WriteCloser
	Stdout      io.ReadCloser
	stdoutWriter io.WriteCloser
	ptyRequested bool
	ptyTerm      string
	ptyCols      uint32
	ptyRows      uint32
	closed       bool
	windowChanges []windowChange
}

type windowChange struct {
	cols, rows int
}

func newMockSession() *mockSession {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	return &mockSession{
		Stdin:        stdinR,
		stdinWriter:  stdinW,
		Stdout:       stdoutR,
		stdoutWriter: stdoutW,
	}
}

func (m *mockSession) RequestPTY(term string, rows, cols int) error {
	m.ptyRequested = true
	m.ptyTerm = term
	m.ptyCols = uint32(cols)
	m.ptyRows = uint32(rows)
	return nil
}

func (m *mockSession) WindowChange(rows, cols int) error {
	m.windowChanges = append(m.windowChanges, windowChange{cols: cols, rows: rows})
	return nil
}

func (m *mockSession) StdinPipe() (io.WriteCloser, error) {
	return m.stdinWriter, nil
}

func (m *mockSession) StdoutPipe() (io.Reader, error) {
	return m.Stdout, nil
}

func (m *mockSession) Start(cmd string) error {
	return nil
}

func (m *mockSession) Shell() error {
	return nil
}

func (m *mockSession) Wait() error {
	// Block until stdout is closed (simulating session end)
	buf := make([]byte, 1)
	for {
		_, err := m.Stdout.Read(buf)
		if err != nil {
			return err
		}
	}
}

func (m *mockSession) Close() error {
	m.closed = true
	m.stdinWriter.Close()
	m.stdoutWriter.Close()
	return nil
}

// mockOpener implements terminal.SessionOpener.
type mockOpener struct {
	sessions map[string]*mockSession
}

func newMockOpener() *mockOpener {
	return &mockOpener{sessions: make(map[string]*mockSession)}
}

func (o *mockOpener) OpenSession(serverID string) (terminal.Session, error) {
	s, ok := o.sessions[serverID]
	if !ok {
		return nil, &terminal.ServerNotFoundError{ServerID: serverID}
	}
	return s, nil
}

func dialTerminalWS(t *testing.T, srv *httptest.Server, serverID string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws/terminal/" + serverID
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}
	return conn
}

func readTextMessage(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Fatalf("expected text message, got type %d", msgType)
	}
	return string(data)
}

func setupTestServer(t *testing.T, opener *mockOpener) (*httptest.Server, *terminal.Handler) {
	t.Helper()
	h := terminal.NewHandler(opener)
	mux := http.NewServeMux()
	mux.Handle("/api/ws/terminal/", h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, h
}

func TestConnectSendsConnectedStatus(t *testing.T) {
	opener := newMockOpener()
	ms := newMockSession()
	opener.sessions["srv1"] = ms

	srv, _ := setupTestServer(t, opener)

	conn := dialTerminalWS(t, srv, "srv1")
	defer conn.Close()

	msg := readTextMessage(t, conn)

	expected := `{"type":"status","status":"connected"}`
	if msg != expected {
		t.Fatalf("expected %s, got %s", expected, msg)
	}

	if !ms.ptyRequested {
		t.Fatal("PTY was not requested")
	}
	if ms.ptyTerm != "xterm-256color" {
		t.Fatalf("expected term xterm-256color, got %s", ms.ptyTerm)
	}
}

func TestClientInputWrittenToSSHStdin(t *testing.T) {
	opener := newMockOpener()
	ms := newMockSession()
	opener.sessions["srv1"] = ms

	srv, _ := setupTestServer(t, opener)

	conn := dialTerminalWS(t, srv, "srv1")
	defer conn.Close()

	readTextMessage(t, conn)

	// Send binary data from client
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("ls -la\n")); err != nil {
		t.Fatalf("write message: %v", err)
	}

	// Read from mock SSH stdin with timeout
	result := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := ms.Stdin.Read(buf)
		result <- string(buf[:n])
	}()

	select {
	case got := <-result:
		if got != "ls -la\n" {
			t.Fatalf("expected 'ls -la\\n', got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout reading from SSH stdin")
	}
}

func TestResizeMessageTriggersWindowChange(t *testing.T) {
	opener := newMockOpener()
	ms := newMockSession()
	opener.sessions["srv1"] = ms

	srv, _ := setupTestServer(t, opener)

	conn := dialTerminalWS(t, srv, "srv1")
	defer conn.Close()

	readTextMessage(t, conn)

	resizeMsg := `{"type":"resize","cols":120,"rows":40}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(resizeMsg)); err != nil {
		t.Fatalf("write resize: %v", err)
	}

	// Give the handler time to process
	time.Sleep(50 * time.Millisecond)

	if len(ms.windowChanges) != 1 {
		t.Fatalf("expected 1 window change, got %d", len(ms.windowChanges))
	}
	wc := ms.windowChanges[0]
	if wc.cols != 120 || wc.rows != 40 {
		t.Fatalf("expected 120x40, got %dx%d", wc.cols, wc.rows)
	}
}

func TestSSHEOFSendsDisconnectedStatus(t *testing.T) {
	opener := newMockOpener()
	ms := newMockSession()
	opener.sessions["srv1"] = ms

	srv, _ := setupTestServer(t, opener)

	conn := dialTerminalWS(t, srv, "srv1")
	defer conn.Close()

	readTextMessage(t, conn)

	// Close SSH stdout to simulate EOF
	ms.stdoutWriter.Close()

	msg := readTextMessage(t, conn)
	expected := `{"type":"status","status":"disconnected","message":"session ended"}`
	if msg != expected {
		t.Fatalf("expected %s, got %s", expected, msg)
	}
}

func TestWebSocketCloseClosesSSHSession(t *testing.T) {
	opener := newMockOpener()
	ms := newMockSession()
	opener.sessions["srv1"] = ms

	srv, _ := setupTestServer(t, opener)

	conn := dialTerminalWS(t, srv, "srv1")
	readTextMessage(t, conn)

	// Close WebSocket from client side
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	conn.Close()

	// Wait for cleanup
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ms.closed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("SSH session was not closed after WebSocket close")
}

func TestInvalidServerIDReturnsError(t *testing.T) {
	opener := newMockOpener()

	srv, _ := setupTestServer(t, opener)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws/terminal/nonexistent"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected error dialing invalid server, got nil")
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", resp.StatusCode)
	}
}

func TestSSHOutputSentAsBinaryFrames(t *testing.T) {
	opener := newMockOpener()
	ms := newMockSession()
	opener.sessions["srv1"] = ms

	srv, _ := setupTestServer(t, opener)

	conn := dialTerminalWS(t, srv, "srv1")
	defer conn.Close()

	// Drain "connected" status
	readTextMessage(t, conn)

	// Write data to mock SSH stdout
	ms.stdoutWriter.Write([]byte("hello from server"))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected binary message, got type %d", msgType)
	}
	if string(data) != "hello from server" {
		t.Fatalf("expected 'hello from server', got %q", string(data))
	}
}
