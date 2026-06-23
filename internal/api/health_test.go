package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"belochka/internal/api"
	"belochka/internal/hub"
)

func TestHealthEndpoint(t *testing.T) {
	h := hub.New()
	router := api.NewRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", contentType)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status field to be \"ok\", got %q", body["status"])
	}
}
