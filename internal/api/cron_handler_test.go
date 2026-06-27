package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"belochka/internal/api"
	"belochka/internal/hub"
)

var errSSHFailed = errors.New("ssh connection refused")

// mockCronExecutor implements api.CronExecutor for testing.
type mockCronExecutor struct {
	output string
	err    error
}

func (m *mockCronExecutor) Execute(_ context.Context, _, _ string) (string, error) {
	return m.output, m.err
}

func setupRouterWithCrons(executor api.CronExecutor) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithCronExecutor(executor))
}

func TestGetCrons_ReturnsParsedEntries(t *testing.T) {
	crontab := "MAILTO=root\n0 * * * * /usr/bin/hourly.sh\n#[disabled] 30 2 * * 0 /usr/bin/weekly.sh\n# comment"
	executor := &mockCronExecutor{output: crontab}
	router := setupRouterWithCrons(executor)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv-1/crons", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("expected entries array in response")
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	passthroughs, ok := result["passthroughs"].([]interface{})
	if !ok {
		t.Fatal("expected passthroughs array in response")
	}
	if len(passthroughs) != 2 {
		t.Fatalf("expected 2 passthroughs, got %d", len(passthroughs))
	}

	// First entry is enabled
	first := entries[0].(map[string]interface{})
	if first["enabled"] != true {
		t.Errorf("first entry should be enabled")
	}
	if first["command"] != "/usr/bin/hourly.sh" {
		t.Errorf("expected command /usr/bin/hourly.sh, got %v", first["command"])
	}

	// Second entry is disabled
	second := entries[1].(map[string]interface{})
	if second["enabled"] != false {
		t.Errorf("second entry should be disabled")
	}
}

func TestGetCrons_SSHError_Returns502(t *testing.T) {
	executor := &mockCronExecutor{err: errSSHFailed}
	router := setupRouterWithCrons(executor)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv-1/crons", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "ssh_error" {
		t.Errorf("expected error code ssh_error, got %v", errObj["code"])
	}
}

func TestGetCrons_EmptyCrontab_ReturnsEmptyArrays(t *testing.T) {
	executor := &mockCronExecutor{output: ""}
	router := setupRouterWithCrons(executor)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv-1/crons", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)

	entries := result["entries"].([]interface{})
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty crontab, got %d", len(entries))
	}
	passthroughs := result["passthroughs"].([]interface{})
	if len(passthroughs) != 0 {
		t.Errorf("expected 0 passthroughs for empty crontab, got %d", len(passthroughs))
	}
}
