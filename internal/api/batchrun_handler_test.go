package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"belochka/internal/api"
	"belochka/internal/batchrun"
	"belochka/internal/model"

	"github.com/gorilla/websocket"
)

// --- Fake service ---

type inputCall struct {
	runID    string
	serverID string
	data     []byte
}

type fakeBatchService struct {
	mu              sync.Mutex
	dispatchScript  string
	dispatchIDs     []string
	dispatchRun     model.BatchRun
	dispatchResults []model.RunResult
	dispatchErr     error
	currentRun      model.BatchRun
	currentResults  []model.RunResult
	currentErr      error
	cancelCalls     int
	inputCalls      []inputCall
	events          chan batchrun.Event
	subscribeErr    error
}

func (f *fakeBatchService) Dispatch(_ context.Context, script string, serverIDs []string) (model.BatchRun, []model.RunResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dispatchScript = script
	f.dispatchIDs = append([]string(nil), serverIDs...)
	return f.dispatchRun, f.dispatchResults, f.dispatchErr
}

func (f *fakeBatchService) Current(_ context.Context) (model.BatchRun, []model.RunResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentRun, f.currentResults, f.currentErr
}

func (f *fakeBatchService) Cancel() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalls++
}

func (f *fakeBatchService) Input(runID, serverID string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputCalls = append(f.inputCalls, inputCall{runID: runID, serverID: serverID, data: append([]byte(nil), data...)})
	return nil
}

func (f *fakeBatchService) Subscribe(runID string) (<-chan batchrun.Event, func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subscribeErr != nil {
		return nil, nil, f.subscribeErr
	}
	return f.events, func() {}, nil
}

func (f *fakeBatchService) gotInputCalls() []inputCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]inputCall(nil), f.inputCalls...)
}

// --- Helpers ---

func newBatchTestServer(t *testing.T, svc api.BatchRunService) *httptest.Server {
	t.Helper()
	router := api.NewRouter(nil, api.WithBatchRunService(svc))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func dialBatchWS(t *testing.T, srv *httptest.Server, runID string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws/batch-runs/" + runID
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}
	return conn
}

func readTextWS(t *testing.T, conn *websocket.Conn) string {
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

func waitForCond(t *testing.T, d time.Duration, cond func() bool, msg string) {
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

// --- REST tests ---

func TestCreateBatchRun(t *testing.T) {
	svc := &fakeBatchService{
		dispatchRun:     model.BatchRun{ID: "run-1", Script: "echo hi", Status: model.BatchRunRunning, CreatedAt: time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)},
		dispatchResults: []model.RunResult{{RunID: "run-1", ServerID: "s1", Status: model.ResultPending}},
	}
	srv := newBatchTestServer(t, svc)

	body := bytes.NewBufferString(`{"script":"echo hi","server_ids":["s1","s2","s1"]}`)
	resp, err := http.Post(srv.URL+"/api/batch-runs", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if svc.dispatchScript != "echo hi" {
		t.Fatalf("expected script 'echo hi', got %q", svc.dispatchScript)
	}
	// Duplicate server IDs are deduped.
	if got := strings.Join(svc.dispatchIDs, ","); got != "s1,s2" {
		t.Fatalf("expected deduped ids s1,s2, got %s", got)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	run := payload["run"].(map[string]any)
	if run["id"] != "run-1" || run["status"] != "running" || run["created_at"] != "2026-08-03T12:00:00Z" {
		t.Fatalf("unexpected run payload: %v", run)
	}
}

func TestCreateBatchRunValidation(t *testing.T) {
	svc := &fakeBatchService{}
	srv := newBatchTestServer(t, svc)

	cases := []struct {
		name string
		body string
		code string
	}{
		{"empty script", `{"script":"","server_ids":["s1"]}`, "validation_failed"},
		{"missing servers", `{"script":"echo hi","server_ids":[]}`, "validation_failed"},
		{"invalid json", `not-json`, "invalid_json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(srv.URL+"/api/batch-runs", "application/json", bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
			var payload map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := payload["error"].(map[string]any)["code"]; got != tc.code {
				t.Fatalf("expected error code %s, got %v", tc.code, got)
			}
		})
	}
}

func TestCreateBatchRunConflictWhileRunning(t *testing.T) {
	svc := &fakeBatchService{dispatchErr: model.ErrBatchRunInProgress}
	srv := newBatchTestServer(t, svc)

	resp, err := http.Post(srv.URL+"/api/batch-runs", "application/json", bytes.NewBufferString(`{"script":"echo hi","server_ids":["s1"]}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := payload["error"].(map[string]any)["code"]; got != "batch_run_in_progress" {
		t.Fatalf("expected code batch_run_in_progress, got %v", got)
	}
}

func TestCurrentBatchRun(t *testing.T) {
	started := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	finished := started.Add(90 * time.Second)
	svc := &fakeBatchService{
		currentRun:     model.BatchRun{ID: "run-1", Script: "echo hi", Status: model.BatchRunDone, CreatedAt: started},
		currentResults: []model.RunResult{{
			RunID: "run-1", ServerID: "s1", Status: model.ResultSuccess,
			ExitCode: intPtr(0), Output: "hi\n", StartedAt: &started, FinishedAt: &finished,
		}},
	}
	srv := newBatchTestServer(t, svc)

	resp, err := http.Get(srv.URL + "/api/batch-runs/current")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	run := payload["run"].(map[string]any)
	if run["status"] != "done" {
		t.Fatalf("expected done run, got %v", run["status"])
	}
	results := payload["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0].(map[string]any)
	if r["status"] != "success" || r["output"] != "hi\n" || r["exit_code"] != float64(0) {
		t.Fatalf("unexpected result payload: %v", r)
	}
	if r["started_at"] != "2026-08-03T12:00:00Z" || r["finished_at"] != "2026-08-03T12:01:30Z" {
		t.Fatalf("unexpected timestamps: %v / %v", r["started_at"], r["finished_at"])
	}
}

func TestCurrentBatchRunNotFound(t *testing.T) {
	svc := &fakeBatchService{currentErr: model.ErrBatchRunNotFound}
	srv := newBatchTestServer(t, svc)

	resp, err := http.Get(srv.URL + "/api/batch-runs/current")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCancelBatchRun(t *testing.T) {
	svc := &fakeBatchService{}
	srv := newBatchTestServer(t, svc)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/batch-runs/current/cancel", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if svc.cancelCalls != 1 {
		t.Fatalf("expected 1 cancel call, got %d", svc.cancelCalls)
	}
}

// --- WebSocket framing tests ---

func TestWSStreamsOutputAndStatusFrames(t *testing.T) {
	svc := &fakeBatchService{events: make(chan batchrun.Event, 16)}
	srv := newBatchTestServer(t, svc)

	conn := dialBatchWS(t, srv, "run-1")
	defer conn.Close()

	svc.events <- batchrun.Event{Type: "output", ServerID: "s1", Data: "hello"}
	code := 3
	svc.events <- batchrun.Event{Type: "status", ServerID: "s1", Status: model.ResultFailed, ExitCode: &code, Error: "exit status 3"}

	// The frames unmarshal back into the service's Event — the service's
	// Event is the wire format (the exact JSON contract is pinned by
	// batchrun's wire tests).
	var got1 batchrun.Event
	if err := json.Unmarshal([]byte(readTextWS(t, conn)), &got1); err != nil {
		t.Fatalf("unmarshal output frame: %v", err)
	}
	if got1.Type != "output" || got1.ServerID != "s1" || got1.Data != "hello" {
		t.Fatalf("unexpected output frame: %+v", got1)
	}

	var got2 batchrun.Event
	if err := json.Unmarshal([]byte(readTextWS(t, conn)), &got2); err != nil {
		t.Fatalf("unmarshal status frame: %v", err)
	}
	if got2.Type != "status" || got2.Status != model.ResultFailed || got2.Error != "exit status 3" {
		t.Fatalf("unexpected status frame: %+v", got2)
	}
	if got2.ExitCode == nil || *got2.ExitCode != 3 {
		t.Fatalf("expected exit code 3, got %v", got2.ExitCode)
	}
}

func TestWSInputFrameDeliveredToService(t *testing.T) {
	svc := &fakeBatchService{events: make(chan batchrun.Event, 4)}
	srv := newBatchTestServer(t, svc)

	conn := dialBatchWS(t, srv, "run-1")
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"input","server_id":"s1","data":"y\n"}`)); err != nil {
		t.Fatalf("write input: %v", err)
	}
	// Non-input frames (e.g. resize) must be ignored, not crash the handler.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80}`)); err != nil {
		t.Fatalf("write other frame: %v", err)
	}

	waitForCond(t, 2*time.Second, func() bool {
		return len(svc.gotInputCalls()) == 1
	}, "input delivered to service")

	calls := svc.gotInputCalls()
	if calls[0].runID != "run-1" || calls[0].serverID != "s1" || string(calls[0].data) != "y\n" {
		t.Fatalf("unexpected input call: %+v", calls[0])
	}
}

func TestWSConnectionClosesWhenStreamEnds(t *testing.T) {
	svc := &fakeBatchService{events: make(chan batchrun.Event, 4)}
	srv := newBatchTestServer(t, svc)

	conn := dialBatchWS(t, srv, "run-1")
	defer conn.Close()

	// The service closing the event stream (run completed) must close the
	// client connection.
	close(svc.events)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected read error after the stream ended")
	}
}

func TestWSRejectsUnknownRun(t *testing.T) {
	svc := &fakeBatchService{subscribeErr: context.DeadlineExceeded}
	srv := newBatchTestServer(t, svc)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws/batch-runs/nope"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial error for unknown run")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func intPtr(v int) *int {
	return &v
}
