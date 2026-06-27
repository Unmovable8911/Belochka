package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// mockCronExecutorFn allows per-call control of the Execute response.
type mockCronExecutorFn struct {
	fn   func(cmd string) (string, error)
	cmds []string
}

func (m *mockCronExecutorFn) Execute(_ context.Context, _, cmd string) (string, error) {
	m.cmds = append(m.cmds, cmd)
	return m.fn(cmd)
}

// decodeBase64FromWriteCmd extracts and decodes the base64 payload from a
// crontab write command produced by createCron: "echo <b64> | base64 -d | crontab -"
func decodeBase64FromWriteCmd(cmd string) (string, error) {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return "", errors.New("unexpected write command format")
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func setupRouterWithCronFn(fn func(cmd string) (string, error)) (http.Handler, *mockCronExecutorFn) {
	exec := &mockCronExecutorFn{fn: fn}
	h := hub.New()
	return api.NewRouter(h, api.WithCronExecutor(exec)), exec
}

func postCron(router http.Handler, serverID string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/"+serverID+"/crons", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateCron_ValidInput_Returns201WithEntry(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return "", nil // empty existing crontab
		}
		return "", nil // write succeeds
	})

	rec := postCron(router, "srv-1", map[string]string{
		"minute":     "30",
		"hour":       "2",
		"dayOfMonth": "*",
		"month":      "*",
		"dayOfWeek":  "0",
		"command":    "/usr/bin/weekly.sh",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var entry map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&entry); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if entry["command"] != "/usr/bin/weekly.sh" {
		t.Errorf("expected command /usr/bin/weekly.sh, got %v", entry["command"])
	}
	if entry["minute"] != "30" || entry["hour"] != "2" {
		t.Errorf("wrong schedule fields: %v %v", entry["minute"], entry["hour"])
	}
	if entry["enabled"] != true {
		t.Errorf("expected enabled=true for new entry")
	}
	if entry["raw"] != "30 2 * * 0 /usr/bin/weekly.sh" {
		t.Errorf("unexpected raw: %v", entry["raw"])
	}
}

func TestCreateCron_PreservesPassthroughLines(t *testing.T) {
	existing := "MAILTO=root\n# comment\n0 * * * * /usr/bin/hourly.sh"
	router, exec := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		return "", nil
	})

	postCron(router, "srv-1", map[string]string{
		"minute":     "30",
		"hour":       "2",
		"dayOfMonth": "*",
		"month":      "*",
		"dayOfWeek":  "0",
		"command":    "/usr/bin/weekly.sh",
	})

	// Find the write command (not the read command)
	var writeCmd string
	for _, cmd := range exec.cmds {
		if !strings.Contains(cmd, "crontab -l") {
			writeCmd = cmd
			break
		}
	}
	if writeCmd == "" {
		t.Fatal("no write command issued")
	}

	written, err := decodeBase64FromWriteCmd(writeCmd)
	if err != nil {
		t.Fatalf("failed to decode write command: %v", err)
	}
	if !strings.Contains(written, "MAILTO=root") {
		t.Errorf("written crontab missing MAILTO passthrough: %q", written)
	}
	if !strings.Contains(written, "# comment") {
		t.Errorf("written crontab missing comment passthrough: %q", written)
	}
	if !strings.Contains(written, "0 * * * * /usr/bin/hourly.sh") {
		t.Errorf("written crontab missing existing entry: %q", written)
	}
	if !strings.Contains(written, "30 2 * * 0 /usr/bin/weekly.sh") {
		t.Errorf("written crontab missing new entry: %q", written)
	}
}

func TestCreateCron_EmptyCommand_Returns400(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(_ string) (string, error) { return "", nil })

	rec := postCron(router, "srv-1", map[string]string{
		"minute":  "*",
		"hour":    "*",
		"command": "",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["code"] != "invalid_input" {
		t.Errorf("expected code invalid_input, got %v", errObj["code"])
	}
}

func TestCreateCron_SSHReadError_Returns502(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return "", errSSHFailed
		}
		return "", nil
	})

	rec := postCron(router, "srv-1", map[string]string{
		"minute":  "*",
		"hour":    "*",
		"command": "/usr/bin/job.sh",
	})

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCron_SSHWriteError_Returns502(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return "", nil
		}
		return "", errSSHFailed
	})

	rec := postCron(router, "srv-1", map[string]string{
		"minute":  "*",
		"hour":    "*",
		"command": "/usr/bin/job.sh",
	})

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}
