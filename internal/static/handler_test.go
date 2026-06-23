package static_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"belochka/internal/static"
)

// testFS builds an in-memory filesystem that mimics web/dist output.
func testFS() fs.FS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<html>belochka</html>")},
		"favicon.svg":          {Data: []byte("<svg/>")},
		"assets/index-abc.js":  {Data: []byte("console.log('app')")},
		"assets/index-abc.css": {Data: []byte("body{}")},
	}
}

func TestServesIndexHTML(t *testing.T) {
	h := static.NewHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "belochka") {
		t.Fatalf("expected index.html content, got %q", rec.Body.String())
	}
}

func TestServesStaticAssets(t *testing.T) {
	h := static.NewHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("expected JS content, got %q", rec.Body.String())
	}
}

func TestServesFavicon(t *testing.T) {
	h := static.NewHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<svg/>") {
		t.Fatalf("expected SVG content, got %q", rec.Body.String())
	}
}

func TestSPAFallback(t *testing.T) {
	h := static.NewHandler(testFS())

	// A path that doesn't match any file should return index.html (SPA routing)
	req := httptest.NewRequest(http.MethodGet, "/server/some-uuid", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "belochka") {
		t.Fatalf("expected index.html fallback, got %q", rec.Body.String())
	}
}

func TestNilFSReturnsNilHandler(t *testing.T) {
	h := static.NewHandler(nil)
	if h != nil {
		t.Fatal("expected nil handler for nil FS")
	}
}
