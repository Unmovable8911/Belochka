package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

// --- PUT /api/servers/{id}/crons/{index} ---

func putCron(router http.Handler, serverID string, index int, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/crons/%d", serverID, index), bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestUpdateCron_ValidInput_Returns200WithUpdatedEntry(t *testing.T) {
	existing := "0 * * * * /usr/bin/hourly.sh\n30 2 * * 0 /usr/bin/weekly.sh\n"
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		return "", nil
	})

	rec := putCron(router, "srv-1", 0, map[string]interface{}{
		"minute":     "15",
		"hour":       "6",
		"dayOfMonth": "*",
		"month":      "*",
		"dayOfWeek":  "*",
		"command":    "/usr/bin/new.sh",
		"enabled":    true,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entry map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&entry); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if entry["command"] != "/usr/bin/new.sh" {
		t.Errorf("expected updated command, got %v", entry["command"])
	}
	if entry["minute"] != "15" || entry["hour"] != "6" {
		t.Errorf("expected updated schedule, got minute=%v hour=%v", entry["minute"], entry["hour"])
	}
	if entry["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", entry["enabled"])
	}
}

func TestUpdateCron_DisabledEntry_WritesDisabledLine(t *testing.T) {
	existing := "0 * * * * /usr/bin/hourly.sh\n"
	var writtenContent string
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		// Capture the written crontab.
		content, err := decodeBase64FromWriteCmd(cmd)
		if err == nil {
			writtenContent = content
		}
		return "", nil
	})

	rec := putCron(router, "srv-1", 0, map[string]interface{}{
		"minute":     "0",
		"hour":       "*",
		"dayOfMonth": "*",
		"month":      "*",
		"dayOfWeek":  "*",
		"command":    "/usr/bin/hourly.sh",
		"enabled":    false,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(writtenContent, "#[disabled] 0 * * * * /usr/bin/hourly.sh") {
		t.Errorf("written content missing disabled line: %q", writtenContent)
	}
}

func TestUpdateCron_PreservesPassthroughs(t *testing.T) {
	existing := "MAILTO=root\n# comment\n0 * * * * /usr/bin/hourly.sh\n30 2 * * 0 /usr/bin/weekly.sh\n"
	var writtenContent string
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		content, err := decodeBase64FromWriteCmd(cmd)
		if err == nil {
			writtenContent = content
		}
		return "", nil
	})

	putCron(router, "srv-1", 0, map[string]interface{}{
		"minute": "*", "hour": "*", "dayOfMonth": "*", "month": "*", "dayOfWeek": "*",
		"command": "/usr/bin/new.sh", "enabled": true,
	})

	if !strings.Contains(writtenContent, "MAILTO=root") {
		t.Errorf("passthrough MAILTO missing: %q", writtenContent)
	}
	if !strings.Contains(writtenContent, "# comment") {
		t.Errorf("passthrough comment missing: %q", writtenContent)
	}
	if !strings.Contains(writtenContent, "30 2 * * 0 /usr/bin/weekly.sh") {
		t.Errorf("other entry missing: %q", writtenContent)
	}
}

func TestUpdateCron_OutOfRange_Returns404(t *testing.T) {
	existing := "0 * * * * /usr/bin/hourly.sh\n"
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		return "", nil
	})

	rec := putCron(router, "srv-1", 5, map[string]interface{}{
		"minute": "*", "hour": "*", "dayOfMonth": "*", "month": "*", "dayOfWeek": "*",
		"command": "/usr/bin/new.sh", "enabled": true,
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateCron_InvalidIndex_Returns400(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) { return "", nil })

	data, _ := json.Marshal(map[string]interface{}{"command": "/usr/bin/x.sh", "enabled": true})
	req := httptest.NewRequest(http.MethodPut, "/api/servers/srv-1/crons/notanumber", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateCron_SSHReadError_Returns502(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return "", errSSHFailed
		}
		return "", nil
	})

	rec := putCron(router, "srv-1", 0, map[string]interface{}{
		"minute": "*", "hour": "*", "dayOfMonth": "*", "month": "*", "dayOfWeek": "*",
		"command": "/usr/bin/new.sh", "enabled": true,
	})

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- DELETE /api/servers/{id}/crons/{index} ---

func deleteCronReq(router http.Handler, serverID string, index int) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/crons/%d", serverID, index), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDeleteCron_ValidIndex_Returns204(t *testing.T) {
	existing := "0 * * * * /usr/bin/hourly.sh\n30 2 * * 0 /usr/bin/weekly.sh\n"
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		return "", nil
	})

	rec := deleteCronReq(router, "srv-1", 0)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteCron_RemovesEntryPreservesPassthroughs(t *testing.T) {
	existing := "MAILTO=root\n0 * * * * /usr/bin/hourly.sh\n30 2 * * 0 /usr/bin/weekly.sh\n"
	var writtenContent string
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		content, err := decodeBase64FromWriteCmd(cmd)
		if err == nil {
			writtenContent = content
		}
		return "", nil
	})

	deleteCronReq(router, "srv-1", 0)

	if strings.Contains(writtenContent, "hourly.sh") {
		t.Errorf("deleted entry should be absent: %q", writtenContent)
	}
	if !strings.Contains(writtenContent, "MAILTO=root") {
		t.Errorf("passthrough should be preserved: %q", writtenContent)
	}
	if !strings.Contains(writtenContent, "30 2 * * 0 /usr/bin/weekly.sh") {
		t.Errorf("other entry should be preserved: %q", writtenContent)
	}
}

func TestDeleteCron_OutOfRange_Returns404(t *testing.T) {
	existing := "0 * * * * /usr/bin/hourly.sh\n"
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return existing, nil
		}
		return "", nil
	})

	rec := deleteCronReq(router, "srv-1", 5)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteCron_SSHReadError_Returns502(t *testing.T) {
	router, _ := setupRouterWithCronFn(func(cmd string) (string, error) {
		if strings.Contains(cmd, "crontab -l") {
			return "", errSSHFailed
		}
		return "", nil
	})

	rec := deleteCronReq(router, "srv-1", 0)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- POST /api/servers/{id}/crons/{index}/run ---

// mockCronRunner implements api.CronRunner for testing.
type mockCronRunner struct {
	output   string
	exitCode int
	err      error
}

func (m *mockCronRunner) RunCommand(_ context.Context, _, _ string) (string, int, error) {
	return m.output, m.exitCode, m.err
}

func runCronReq(router http.Handler, serverID string, index int) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/crons/%d/run", serverID, index), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func setupRouterWithRunner(executor api.CronExecutor, runner api.CronRunner) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithCronExecutor(executor), api.WithCronRunner(runner))
}

func TestRunCron_ValidIndex_Returns200WithOutputAndExitCode(t *testing.T) {
	executor := &mockCronExecutor{output: "0 * * * * /usr/bin/hourly.sh\n"}
	runner := &mockCronRunner{output: "hello\n", exitCode: 0}
	router := setupRouterWithRunner(executor, runner)

	rec := runCronReq(router, "srv-1", 0)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["exitCode"].(float64) != 0 {
		t.Errorf("expected exitCode 0, got %v", result["exitCode"])
	}
	if result["output"] != "hello\n" {
		t.Errorf("expected output 'hello\\n', got %v", result["output"])
	}
}

func TestRunCron_NonZeroExitCode_Returns200(t *testing.T) {
	executor := &mockCronExecutor{output: "0 * * * * /usr/bin/fail.sh\n"}
	runner := &mockCronRunner{output: "error output\n", exitCode: 1}
	router := setupRouterWithRunner(executor, runner)

	rec := runCronReq(router, "srv-1", 0)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even for non-zero exit, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	if result["exitCode"].(float64) != 1 {
		t.Errorf("expected exitCode 1, got %v", result["exitCode"])
	}
	if result["output"] != "error output\n" {
		t.Errorf("expected output 'error output\\n', got %v", result["output"])
	}
}

func TestRunCron_InvalidIndex_Returns400(t *testing.T) {
	executor := &mockCronExecutor{output: ""}
	runner := &mockCronRunner{}
	h := hub.New()
	router := api.NewRouter(h, api.WithCronExecutor(executor), api.WithCronRunner(runner))

	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv-1/crons/notanumber/run", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunCron_OutOfRange_Returns404(t *testing.T) {
	executor := &mockCronExecutor{output: "0 * * * * /usr/bin/hourly.sh\n"}
	runner := &mockCronRunner{}
	router := setupRouterWithRunner(executor, runner)

	rec := runCronReq(router, "srv-1", 5)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunCron_SSHReadError_Returns502(t *testing.T) {
	executor := &mockCronExecutor{err: errSSHFailed}
	runner := &mockCronRunner{}
	router := setupRouterWithRunner(executor, runner)

	rec := runCronReq(router, "srv-1", 0)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRunCron_SSHExecuteError_Returns502(t *testing.T) {
	executor := &mockCronExecutor{output: "0 * * * * /usr/bin/hourly.sh\n"}
	runner := &mockCronRunner{err: errSSHFailed}
	router := setupRouterWithRunner(executor, runner)

	rec := runCronReq(router, "srv-1", 0)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}
