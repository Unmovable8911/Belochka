package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"belochka/internal/api"
	"belochka/internal/hub"
	"belochka/internal/process"
)

func setupRouterWithProcessService(svc *process.Service) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithProcessService(svc))
}

// Test helper: create a process service with a stub executor
func makeProcessService(output string, err error) *process.Service {
	return process.NewService(&stubProcessExecutor{output: output, err: err})
}

type stubProcessExecutor struct {
	output string
	err    error
}

func (s *stubProcessExecutor) Execute(_ context.Context, _, _ string) (string, error) {
	return s.output, s.err
}

// --- List tests ---

func TestListProcesses_ReturnsProcesses(t *testing.T) {
	psOutput := "1 0 root 1000 0.0 0.1 01:00:00 systemd\n2 0 root 2000 1.5 0.3 00:30:00 sshd"
	svc := makeProcessService(psOutput, nil)
	router := setupRouterWithProcessService(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv-1/processes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var procs []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&procs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(procs))
	}
	if procs[0]["pid"] != float64(1) {
		t.Errorf("first pid = %v, want 1", procs[0]["pid"])
	}
	if procs[0]["ppid"] != float64(0) {
		t.Errorf("first ppid = %v, want 0", procs[0]["ppid"])
	}
}

func TestListProcesses_EmptyList(t *testing.T) {
	svc := makeProcessService("", nil)
	router := setupRouterWithProcessService(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv-1/processes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var procs []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&procs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(procs) != 0 {
		t.Errorf("expected empty array, got %d entries", len(procs))
	}
}

func TestListProcesses_SSHError(t *testing.T) {
	svc := makeProcessService("", errors.New("ssh connection refused"))
	router := setupRouterWithProcessService(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv-1/processes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	errObj := body["error"].(map[string]interface{})
	if errObj["code"] != "ssh_error" {
		t.Errorf("expected ssh_error code, got %v", errObj["code"])
	}
}

// --- Kill tests ---

func TestKillProcess_Success(t *testing.T) {
	// Use stub executor that returns "myapp" then success
	exec := &stubProcessExecutor{output: "myapp"}
	svc := process.NewService(exec)
	router := setupRouterWithProcessService(svc)

	body := strings.NewReader(`{"signal":"SIGTERM"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv-1/processes/1234/kill", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success: true")
	}
	if resp["pid"] != float64(1234) {
		t.Errorf("expected pid 1234, got %v", resp["pid"])
	}
}

func TestKillProcess_DefaultSignal(t *testing.T) {
	exec := &stubProcessExecutor{output: "myapp"}
	svc := process.NewService(exec)
	router := setupRouterWithProcessService(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv-1/processes/1234/kill", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["signal"] != "SIGTERM" {
		t.Errorf("expected default signal SIGTERM, got %v", resp["signal"])
	}
}

func TestKillProcess_ProtectedProcess(t *testing.T) {
	exec := &stubProcessExecutor{output: "sshd"}
	svc := process.NewService(exec)
	router := setupRouterWithProcessService(svc)

	body := strings.NewReader(`{"signal":"SIGTERM"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv-1/processes/5678/kill", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	var body2 map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body2)
	errObj := body2["error"].(map[string]interface{})
	if errObj["code"] != "protected_process" {
		t.Errorf("expected protected_process code, got %v", errObj["code"])
	}
}

func TestKillProcess_ProcessNotFound(t *testing.T) {
	exec := &stubProcessExecutor{output: ""}
	svc := process.NewService(exec)
	router := setupRouterWithProcessService(svc)

	body := strings.NewReader(`{"signal":"SIGTERM"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv-1/processes/9999/kill", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestKillProcess_InvalidPID(t *testing.T) {
	exec := &stubProcessExecutor{}
	svc := process.NewService(exec)
	router := setupRouterWithProcessService(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv-1/processes/abc/kill", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
