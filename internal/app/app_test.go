package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"belochka/internal/app"
	"belochka/internal/config"
)

func TestApplicationLifecycle(t *testing.T) {
	cfg := config.Config{
		Port:    0, // OS-assigned port
		DataDir: filepath.Join(t.TempDir(), "data"),
	}

	a, err := app.New(cfg, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := a.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer a.Shutdown()

	resp, err := http.Get("http://" + a.Addr() + "/api/health")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}
