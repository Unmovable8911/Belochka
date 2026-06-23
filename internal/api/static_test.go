package api_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"belochka/internal/api"
	"belochka/internal/hub"
)

func staticTestFS() fs.FS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<html>belochka-app</html>")},
		"favicon.svg":          {Data: []byte("<svg/>")},
		"assets/index-abc.js":  {Data: []byte("console.log('app')")},
		"assets/index-abc.css": {Data: []byte("body{}")},
	}
}

func TestAPIRoutesNotAffectedByStaticServing(t *testing.T) {
	h := hub.New()
	router := api.NewRouter(h, api.WithStaticFS(staticTestFS()))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestNonAPIRoutesServeStaticFiles(t *testing.T) {
	h := hub.New()
	router := api.NewRouter(h, api.WithStaticFS(staticTestFS()))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "belochka-app") {
		t.Fatalf("expected embedded index.html, got %q", rec.Body.String())
	}
}

func TestSPAFallbackOnRouter(t *testing.T) {
	h := hub.New()
	router := api.NewRouter(h, api.WithStaticFS(staticTestFS()))

	req := httptest.NewRequest(http.MethodGet, "/server/some-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "belochka-app") {
		t.Fatalf("expected SPA fallback to index.html, got %q", rec.Body.String())
	}
}

func TestStaticServesAssets(t *testing.T) {
	h := hub.New()
	router := api.NewRouter(h, api.WithStaticFS(staticTestFS()))

	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc.js", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("expected JS content, got %q", rec.Body.String())
	}
}

func TestNoStaticFSSkipsStaticServing(t *testing.T) {
	h := hub.New()
	// No WithStaticFS option — development mode.
	router := api.NewRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Without static FS, non-API routes should 404.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without static FS, got %d", rec.Code)
	}
}
