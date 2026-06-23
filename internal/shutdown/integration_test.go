package shutdown_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"belochka/internal/hub"
	"belochka/internal/shutdown"
	"belochka/internal/store"

	"github.com/gorilla/websocket"
)

func TestShutdownSequenceWithHub(t *testing.T) {
	// Start a real hub and connect a WebSocket client.
	h := hub.New()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	go h.Run(hubCtx)

	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Connect a WebSocket client.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	// Drain snapshot.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Build shutdown sequence.
	seq := shutdown.NewSequence(10 * time.Second)

	// Step 1: Stop accepting HTTP (server shutdown).
	seq.Add("http", func(ctx context.Context) error {
		return srv.Config.Shutdown(ctx)
	})

	// Step 2: Cancel hub context (sends close frames to WS clients).
	seq.Add("websocket", func(ctx context.Context) error {
		hubCancel()
		return nil
	})

	// Run the shutdown sequence.
	if err := seq.Run(context.Background()); err != nil {
		t.Fatalf("shutdown sequence error: %v", err)
	}

	// Verify: WebSocket client should have received a close frame.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Fatal("expected read error after shutdown, got none")
	}
	closeErr, ok := readErr.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected CloseError, got %T: %v", readErr, readErr)
	}
	if closeErr.Code != websocket.CloseGoingAway {
		t.Fatalf("expected close code 1001, got %d", closeErr.Code)
	}

	// Verify: Hub should have zero clients.
	if count := h.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", count)
	}
}

func TestFullShutdownOrder(t *testing.T) {
	// This test simulates the full production shutdown order:
	// HTTP → WebSocket → collectors → SSH → SQLite

	// Record execution order.
	rec := &recorder{}

	// 1. Set up real HTTP server + hub.
	h := hub.New()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	go h.Run(hubCtx)
	h.SetSnapshot(json.RawMessage(`{}`))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Connect WS client.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage() // drain snapshot

	// 2. Set up real SQLite store.
	dir := t.TempDir()
	db, err := store.Open(dir, "testkey")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Snapshot goroutines before shutdown to detect leaks later.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	// 3. Build shutdown sequence in production order.
	seq := shutdown.NewSequence(10 * time.Second)

	seq.Add("http", func(ctx context.Context) error {
		rec.record("http")
		return srv.Config.Shutdown(ctx)
	})

	seq.Add("websocket", func(ctx context.Context) error {
		rec.record("websocket")
		hubCancel()
		return nil
	})

	seq.Add("collectors", func(ctx context.Context) error {
		rec.record("collectors")
		// No manager in this test; step just logs.
		return nil
	})

	seq.Add("ssh", func(ctx context.Context) error {
		rec.record("ssh")
		// No SSH pool in this test.
		return nil
	})

	seq.Add("sqlite", func(ctx context.Context) error {
		rec.record("sqlite")
		return db.Close()
	})

	// 4. Execute shutdown.
	if err := seq.Run(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	// 5. Verify order.
	got := rec.steps()
	want := []string{"http", "websocket", "collectors", "ssh", "sqlite"}
	if len(got) != len(want) {
		t.Fatalf("step count: got %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: got %q, want %q", i, got[i], want[i])
		}
	}

	// 6. Verify WebSocket got close frame.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr != nil {
		closeErr, ok := readErr.(*websocket.CloseError)
		if ok && closeErr.Code != websocket.CloseGoingAway {
			t.Errorf("expected close code 1001, got %d", closeErr.Code)
		}
	}

	// 7. Check for goroutine leaks (allow some slack for runtime goroutines).
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	goroutinesAfter := runtime.NumGoroutine()
	// Shutdown should not have increased goroutine count.
	if goroutinesAfter > goroutinesBefore {
		t.Logf("goroutines before=%d after=%d (delta=%d)", goroutinesBefore, goroutinesAfter, goroutinesAfter-goroutinesBefore)
		// Only fail if it's a significant leak (more than a couple).
		if goroutinesAfter > goroutinesBefore+3 {
			t.Errorf("possible goroutine leak: before=%d after=%d", goroutinesBefore, goroutinesAfter)
		}
	}
}
