package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"belochka/internal/api"
)

func TestServerListensOnConfiguredPort(t *testing.T) {
	router := api.NewRouter()

	srv := &http.Server{
		Addr:    ":53136",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("server failed to start: %v", err)
		}
	}()
	defer srv.Close()

	// Give the server a moment to bind.
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get("http://localhost:53136/api/health")
	if err != nil {
		t.Fatalf("failed to reach server on port 53136: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}
