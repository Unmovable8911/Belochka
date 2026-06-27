package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"belochka/internal/api"
	"belochka/internal/hub"
	"belochka/internal/model"
	"belochka/internal/ssh"
)

// mockStore implements api.ServerStore for testing.
type mockStore struct {
	servers map[string]model.Server
	nextID  int
}

func newMockStore() *mockStore {
	return &mockStore{servers: make(map[string]model.Server)}
}

func (m *mockStore) Create(_ context.Context, srv model.Server) (model.Server, error) {
	m.nextID++
	srv.ID = fmt.Sprintf("test-id-%d", m.nextID)
	srv.Password = "" // store clears password from return value
	m.servers[srv.ID] = srv
	return srv, nil
}

func (m *mockStore) GetByID(_ context.Context, id string) (model.Server, error) {
	srv, ok := m.servers[id]
	if !ok {
		return model.Server{}, fmt.Errorf("%w: %s", model.ErrServerNotFound, id)
	}
	return srv, nil
}

func (m *mockStore) List(_ context.Context) ([]model.Server, error) {
	var result []model.Server
	for _, srv := range m.servers {
		result = append(result, srv)
	}
	return result, nil
}

func (m *mockStore) Update(_ context.Context, srv model.Server) (model.Server, error) {
	existing, ok := m.servers[srv.ID]
	if !ok {
		return model.Server{}, fmt.Errorf("%w: %s", model.ErrServerNotFound, srv.ID)
	}
	srv.CreatedAt = existing.CreatedAt
	if srv.Password == "" {
		srv.Password = existing.Password
	}
	m.servers[srv.ID] = srv
	return srv, nil
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	if _, ok := m.servers[id]; !ok {
		return fmt.Errorf("%w: %s", model.ErrServerNotFound, id)
	}
	delete(m.servers, id)
	return nil
}

// mockSSHTester implements api.SSHTester for testing.
type mockSSHTester struct {
	result ssh.TestResult
	err    error
	gotSrv model.Server // captures the last server passed to TestConnection
}

func (m *mockSSHTester) TestConnection(srv model.Server) (ssh.TestResult, error) {
	m.gotSrv = srv
	return m.result, m.err
}

func setupRouter(store api.ServerStore) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithServerStore(store))
}

func setupRouterWithSSH(store api.ServerStore, tester api.SSHTester) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithServerStore(store), api.WithSSHTester(tester))
}

func TestListServers_ReturnsAllWithoutPasswords(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	// Create two servers via the API
	for _, name := range []string{"web-1", "web-2"} {
		body, _ := json.Marshal(map[string]interface{}{
			"name": name, "host": "10.0.0.1", "port": 22,
			"username": "deploy", "password": "secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("setup: expected 201, got %d", rec.Code)
		}
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var servers []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&servers); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	for _, srv := range servers {
		if _, exists := srv["password"]; exists {
			t.Fatal("password must not appear in list response")
		}
	}
}

func TestGetServer_ReturnsServerWithoutPassword(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	// Create a server
	body, _ := json.Marshal(map[string]interface{}{
		"name": "db-1", "host": "10.0.0.5", "port": 22,
		"username": "admin", "password": "dbpass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Get by ID
	req = httptest.NewRequest(http.MethodGet, "/api/servers/"+id, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var srv map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&srv)

	if srv["name"] != "db-1" {
		t.Fatalf("expected name db-1, got %v", srv["name"])
	}
	if _, exists := srv["password"]; exists {
		t.Fatal("password must not appear in get response")
	}
}

func TestGetServer_NotFound(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "not_found" {
		t.Fatalf("expected error code not_found, got %v", errObj["code"])
	}
}

func TestUpdateServer_UpdatesFieldsAndKeepsPassword(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	// Create a server with a password
	body, _ := json.Marshal(map[string]interface{}{
		"name": "old-name", "host": "10.0.0.1", "port": 22,
		"username": "deploy", "password": "original-pass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Update with empty password (should keep original)
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name": "new-name", "host": "10.0.0.2", "port": 2222,
		"username": "deployer", "password": "",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/servers/"+id, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&updated)

	if updated["name"] != "new-name" {
		t.Fatalf("expected name new-name, got %v", updated["name"])
	}
	if updated["host"] != "10.0.0.2" {
		t.Fatalf("expected host 10.0.0.2, got %v", updated["host"])
	}
	if _, exists := updated["password"]; exists {
		t.Fatal("password must not appear in update response")
	}
}

func TestUpdateServer_NotFound(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "x", "host": "x", "username": "x",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/servers/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteServer_RemovesServer(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	// Create a server
	body, _ := json.Marshal(map[string]interface{}{
		"name": "temp", "host": "10.0.0.1", "port": 22, "username": "user",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Delete it
	req = httptest.NewRequest(http.MethodDelete, "/api/servers/"+id, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify it's gone
	req = httptest.NewRequest(http.MethodGet, "/api/servers/"+id, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestDeleteServer_NotFound(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/servers/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateServer_ValidationErrors(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing name", map[string]interface{}{"host": "10.0.0.1", "username": "user"}},
		{"missing host", map[string]interface{}{"name": "web", "username": "user"}},
		{"missing username", map[string]interface{}{"name": "web", "host": "10.0.0.1"}},
		{"all empty", map[string]interface{}{}},
		{"whitespace name", map[string]interface{}{"name": "  ", "host": "10.0.0.1", "username": "user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			json.NewDecoder(rec.Body).Decode(&resp)

			errObj, ok := resp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("expected error object in response")
			}
			if errObj["code"] != "validation_failed" {
				t.Fatalf("expected error code validation_failed, got %v", errObj["code"])
			}
		})
	}
}

func TestUpdateServer_ValidationErrors(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	// Create a server first
	body, _ := json.Marshal(map[string]interface{}{
		"name": "ok", "host": "10.0.0.1", "port": 22, "username": "user",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Try to update with missing name
	updateBody, _ := json.Marshal(map[string]interface{}{
		"host": "10.0.0.2", "username": "user",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/servers/"+id, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateServer_InvalidJSON(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "invalid_json" {
		t.Fatalf("expected error code invalid_json, got %v", errObj["code"])
	}
}

func TestCreateServer_ReturnsCreatedWithoutPassword(t *testing.T) {
	store := newMockStore()
	router := setupRouter(store)

	body := map[string]interface{}{
		"name":     "prod-web-1",
		"host":     "192.168.1.10",
		"port":     22,
		"username": "deploy",
		"password": "secret123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["name"] != "prod-web-1" {
		t.Fatalf("expected name prod-web-1, got %v", resp["name"])
	}
	if resp["host"] != "192.168.1.10" {
		t.Fatalf("expected host 192.168.1.10, got %v", resp["host"])
	}
	if _, exists := resp["password"]; exists {
		t.Fatal("password must not appear in response")
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Fatal("expected non-empty id in response")
	}
}


// postTestConnection POSTs a server config body to the stateless test endpoint.
func postTestConnection(router http.Handler, body map[string]interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/test", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// validTestBody returns a complete, valid server config for the test endpoint.
func validTestBody() map[string]interface{} {
	return map[string]interface{}{
		"name": "web-1", "host": "10.0.0.1", "port": 22,
		"username": "deploy", "auth_type": "password", "password": "secret",
	}
}

func TestTestServer_Success_ReturnsFingerprint(t *testing.T) {
	tester := &mockSSHTester{result: ssh.TestResult{Fingerprint: "SHA256:abc123"}}
	router := setupRouterWithSSH(newMockStore(), tester)

	rec := postTestConnection(router, validTestBody())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["fingerprint"] != "SHA256:abc123" {
		t.Fatalf("expected fingerprint SHA256:abc123, got %v", resp["fingerprint"])
	}
}

func TestTestServer_ValidationError_Returns400(t *testing.T) {
	tester := &mockSSHTester{}
	router := setupRouterWithSSH(newMockStore(), tester)

	rec := postTestConnection(router, map[string]interface{}{"port": 22})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "validation_failed" {
		t.Fatalf("expected validation_failed, got %v", errObj["code"])
	}
}

// When the password is omitted for an existing server (id present), the
// endpoint reuses the stored secret for the test without persisting anything.
func TestTestServer_ReusesStoredPasswordWhenOmitted(t *testing.T) {
	store := newMockStore()
	store.servers["srv-1"] = model.Server{
		ID: "srv-1", Name: "web-1", Host: "10.0.0.1", Port: 22,
		Username: "deploy", AuthType: model.AuthTypePassword, Password: "stored-secret",
	}
	tester := &mockSSHTester{result: ssh.TestResult{Fingerprint: "SHA256:fp"}}
	router := setupRouterWithSSH(store, tester)

	// Changed host, password omitted.
	rec := postTestConnection(router, map[string]interface{}{
		"id": "srv-1", "name": "web-1", "host": "10.0.0.9", "port": 22,
		"username": "deploy", "auth_type": "password",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if tester.gotSrv.Password != "stored-secret" {
		t.Fatalf("expected stored password to be reused, got %q", tester.gotSrv.Password)
	}
}

func TestTestServer_AuthFailure_Returns422(t *testing.T) {
	tester := &mockSSHTester{
		err: &ssh.ConnectionError{
			Kind:    ssh.ErrAuth,
			Message: "authentication failed",
		},
	}
	router := setupRouterWithSSH(newMockStore(), tester)

	rec := postTestConnection(router, validTestBody())

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "auth_failed" {
		t.Fatalf("expected error code auth_failed, got %v", errObj["code"])
	}
}

func TestTestServer_HostKeyMismatch_Returns422(t *testing.T) {
	tester := &mockSSHTester{
		err: &ssh.ConnectionError{
			Kind:    ssh.ErrHostKey,
			Message: "host key mismatch",
		},
	}
	router := setupRouterWithSSH(newMockStore(), tester)

	rec := postTestConnection(router, validTestBody())

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "host_key_mismatch" {
		t.Fatalf("expected error code host_key_mismatch, got %v", errObj["code"])
	}
}

func TestTestServer_NetworkError_Returns422(t *testing.T) {
	tester := &mockSSHTester{
		err: &ssh.ConnectionError{
			Kind:    ssh.ErrNetwork,
			Message: "connection refused",
		},
	}
	router := setupRouterWithSSH(newMockStore(), tester)

	rec := postTestConnection(router, validTestBody())

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "network_error" {
		t.Fatalf("expected error code network_error, got %v", errObj["code"])
	}
}

func TestTestServer_PassphraseKey_Returns422(t *testing.T) {
	tester := &mockSSHTester{
		err: &ssh.ConnectionError{
			Kind:    ssh.ErrPassphrase,
			Message: "key file is passphrase-protected; passphrase-protected keys are not supported",
		},
	}
	router := setupRouterWithSSH(newMockStore(), tester)

	rec := postTestConnection(router, validTestBody())

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "passphrase_protected_key" {
		t.Fatalf("expected error code passphrase_protected_key, got %v", errObj["code"])
	}
}
