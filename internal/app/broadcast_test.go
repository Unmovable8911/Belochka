package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"belochka/internal/app"
	"belochka/internal/hub"
	"belochka/internal/model"
	"belochka/internal/ssh"

	"github.com/gorilla/websocket"
)

// The BroadcastService is tested through its real adapter: a real Hub with
// real WebSocket connections (same pattern as hub_test). Only the data
// sources behind the narrow interfaces are stubbed.

// stubLister records whether ListWithoutPasswords was called.
type stubLister struct {
	servers []model.Server
	err     error
	called  bool
}

func (s *stubLister) ListWithoutPasswords(ctx context.Context) ([]model.Server, error) {
	s.called = true
	return s.servers, s.err
}

type stubStatusProvider struct{ status ssh.ConnStatus }

func (s *stubStatusProvider) Status(serverID string) ssh.ConnStatus { return s.status }

type stubSnapshotProvider struct{ snapshots map[string]*model.Snapshot }

func (s *stubSnapshotProvider) Latest(serverID string) *model.Snapshot {
	return s.snapshots[serverID]
}

// --- WebSocket harness ---

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

// waitForClients polls until the hub has registered the expected number of
// clients. Registration is async after the HTTP upgrade, and Broadcast skips
// entirely with zero clients, so a Broadcast issued right after dialWS could
// see 0 clients and deliver nothing.
func waitForClients(t *testing.T, h *hub.Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for h.ClientCount() != want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := h.ClientCount(); got != want {
		t.Fatalf("expected %d clients registered, got %d", want, got)
	}
}

// readEnvelope reads one message and decodes it as a hub.Envelope.
func readEnvelope(t *testing.T, conn *websocket.Conn) hub.Envelope {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	var env hub.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return env
}

// snapshotPayload decodes a snapshot envelope's data into its wire shape.
type snapshotPayload struct {
	Servers []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"status"`
	} `json:"servers"`
	Metrics map[string]*model.Snapshot `json:"metrics"`
}

func decodeSnapshot(t *testing.T, env hub.Envelope) snapshotPayload {
	t.Helper()
	if env.Type != "snapshot" {
		t.Fatalf("expected type snapshot, got %q", env.Type)
	}
	var payload snapshotPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal snapshot data: %v", err)
	}
	return payload
}

// harness wires a real Hub behind a BroadcastService with stubbed data
// sources. The Hub's ServeWS is exposed on an httptest.Server.
type harness struct {
	hub     *hub.Hub
	cancel  context.CancelFunc
	srv     *httptest.Server
	service *app.BroadcastService
	lister  *stubLister
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := hub.New()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	lister := &stubLister{servers: []model.Server{
		{ID: "srv1", Name: "web-1", Host: "10.0.0.1"},
	}}
	status := &stubStatusProvider{status: ssh.ConnStatus{State: ssh.StateConnected}}
	snapshots := &stubSnapshotProvider{snapshots: map[string]*model.Snapshot{
		"srv1": {ServerID: "srv1", System: model.SystemInfo{Hostname: "web-1"}},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.ServeWS)
	srv := httptest.NewServer(mux)

	return &harness{
		hub:     h,
		cancel:  cancel,
		srv:     srv,
		service: app.NewBroadcastService(lister, status, snapshots, h),
		lister:  lister,
	}
}

func (hs *harness) close() {
	hs.srv.Close()
	hs.cancel()
}

func TestBroadcastDeliversToAllClients(t *testing.T) {
	hs := newHarness(t)
	defer hs.close()

	conn1 := dialWS(t, hs.srv)
	defer conn1.Close()
	conn2 := dialWS(t, hs.srv)
	defer conn2.Close()
	waitForClients(t, hs.hub, 2)

	hs.service.Broadcast(context.Background())

	for _, conn := range []*websocket.Conn{conn1, conn2} {
		payload := decodeSnapshot(t, readEnvelope(t, conn))
		if len(payload.Servers) != 1 || payload.Servers[0].ID != "srv1" || payload.Servers[0].Name != "web-1" || payload.Servers[0].State != "connected" {
			t.Errorf("server payload mismatch: %+v", payload.Servers)
		}
		if payload.Metrics["srv1"] == nil {
			t.Error("missing metrics for srv1")
		}
	}
}

func TestBroadcastSnapshotCachedForLateJoiner(t *testing.T) {
	hs := newHarness(t)
	defer hs.close()

	conn1 := dialWS(t, hs.srv)
	defer conn1.Close()
	waitForClients(t, hs.hub, 1)

	hs.service.Broadcast(context.Background())
	first := readEnvelope(t, conn1)

	// A client joining after the broadcast receives the cached snapshot
	// immediately on connect, with the same data.
	conn2 := dialWS(t, hs.srv)
	defer conn2.Close()

	env := readEnvelope(t, conn2)
	if env.Type != "snapshot" {
		t.Fatalf("expected type snapshot, got %q", env.Type)
	}
	if string(env.Data) != string(first.Data) {
		t.Errorf("late joiner snapshot mismatch:\n got  %s\n want %s", env.Data, first.Data)
	}
}

func TestBroadcastSkipsWhenNoClients(t *testing.T) {
	hs := newHarness(t)
	defer hs.close()

	// No client connected: Broadcast returns without touching the store.
	hs.service.Broadcast(context.Background())

	if hs.lister.called {
		t.Fatal("ListWithoutPasswords should not be called with zero clients connected")
	}
}

func TestBroadcastKeepsDeliveringAfterClientDisconnect(t *testing.T) {
	hs := newHarness(t)
	defer hs.close()

	conn1 := dialWS(t, hs.srv)
	defer conn1.Close()
	conn2 := dialWS(t, hs.srv)
	defer conn2.Close()
	waitForClients(t, hs.hub, 2)

	hs.service.Broadcast(context.Background())
	readEnvelope(t, conn1)
	readEnvelope(t, conn2)

	// Disconnect conn1 and wait for the hub to process the unregistration.
	conn1.Close()
	waitForClients(t, hs.hub, 1)

	hs.service.Broadcast(context.Background())
	payload := decodeSnapshot(t, readEnvelope(t, conn2))
	if len(payload.Servers) != 1 || payload.Servers[0].ID != "srv1" {
		t.Errorf("server payload mismatch: %+v", payload.Servers)
	}
}
