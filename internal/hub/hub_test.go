package hub_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"belochka/internal/hub"

	"github.com/gorilla/websocket"
)

// dialWS upgrades an httptest.Server to a WebSocket connection.
func dialWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}
	return conn
}

func TestSnapshotSentOnConnect(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	snapshot := json.RawMessage(`{"servers":[]}`)
	h.SetSnapshot(snapshot)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	var env hub.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if env.Type != "snapshot" {
		t.Fatalf("expected type snapshot, got %q", env.Type)
	}

	if string(env.Data) != string(snapshot) {
		t.Fatalf("expected snapshot data %s, got %s", snapshot, env.Data)
	}
}

func TestBroadcastMetricsToAllClients(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Set empty snapshot so connect snapshot is trivial.
	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Connect two clients.
	conn1 := dialWS(t, srv)
	defer conn1.Close()
	conn2 := dialWS(t, srv)
	defer conn2.Close()

	// Drain snapshot messages from both.
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn1.ReadMessage()
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn2.ReadMessage()

	// Broadcast a metrics message.
	metricsData := json.RawMessage(`{"cpu":42}`)
	h.BroadcastMsg("metrics", metricsData)

	// Both clients should receive it.
	for i, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d: read: %v", i, err)
		}
		var env hub.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("client %d: unmarshal: %v", i, err)
		}
		if env.Type != "metrics" {
			t.Fatalf("client %d: expected type metrics, got %q", i, env.Type)
		}
		if string(env.Data) != string(metricsData) {
			t.Fatalf("client %d: expected data %s, got %s", i, metricsData, env.Data)
		}
	}
}

func TestBroadcastStatusMessage(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv)
	defer conn.Close()

	// Drain snapshot.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Broadcast a status message.
	statusData := json.RawMessage(`{"server_id":"abc","state":"disconnected"}`)
	h.BroadcastMsg("status", statusData)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var env hub.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Type != "status" {
		t.Fatalf("expected type status, got %q", env.Type)
	}
	if string(env.Data) != string(statusData) {
		t.Fatalf("expected data %s, got %s", statusData, env.Data)
	}
}

func TestConnectionLimitEnforced(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Connect 10 clients (the limit).
	conns := make([]*websocket.Conn, 10)
	for i := 0; i < 10; i++ {
		conn := dialWS(t, srv)
		defer conn.Close()
		conns[i] = conn
		// Drain snapshot.
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
	}

	// 11th connection should be rejected with close code 1013.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"
	conn11, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// If connection was refused entirely, check error for close code.
		if closeErr, ok := err.(*websocket.CloseError); ok {
			if closeErr.Code != 1013 {
				t.Fatalf("expected close code 1013, got %d", closeErr.Code)
			}
			return
		}
		t.Fatalf("unexpected error type: %v", err)
	}
	defer conn11.Close()

	// If dial succeeded, we should receive a close frame when we try to read.
	conn11.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn11.ReadMessage()
	if err == nil {
		t.Fatal("expected error reading from 11th connection, got none")
	}
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected CloseError, got %T: %v", err, err)
	}
	if closeErr.Code != 1013 {
		t.Fatalf("expected close code 1013, got %d", closeErr.Code)
	}
}

func TestCleanClientRemovalOnDisconnect(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv)

	// Drain snapshot.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Verify client is registered.
	// Give the hub event loop a moment to process registration.
	time.Sleep(50 * time.Millisecond)
	if count := h.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}

	// Close the client connection.
	conn.Close()

	// Wait for the hub to process unregistration.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.ClientCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if count := h.ClientCount(); count != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", count)
	}
}

func TestDisconnectFreesSlotForNewConnection(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Fill up to the limit.
	conns := make([]*websocket.Conn, 10)
	for i := 0; i < 10; i++ {
		conn := dialWS(t, srv)
		conns[i] = conn
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
	}

	// Disconnect one client.
	conns[0].Close()

	// Wait for the hub to process the unregistration.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.ClientCount() < 10 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A new connection should now succeed.
	newConn := dialWS(t, srv)
	defer newConn.Close()

	newConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := newConn.ReadMessage()
	if err != nil {
		t.Fatalf("read snapshot from new connection: %v", err)
	}

	var env hub.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != "snapshot" {
		t.Fatalf("expected snapshot, got %q", env.Type)
	}

	// Clean up remaining connections.
	for i := 1; i < 10; i++ {
		conns[i].Close()
	}
}

func TestGracefulShutdownClosesClients(t *testing.T) {
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv)
	defer conn.Close()

	// Drain snapshot.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Cancel the hub context (simulates graceful shutdown).
	cancel()

	// The client should get disconnected: a read should fail.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected read error after hub shutdown, got none")
	}
}
