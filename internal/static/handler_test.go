package static_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"belochka/internal/static"
)

const langPlaceholder = `<meta name="app-lang" content="">`

// testFS builds an in-memory filesystem that mimics web/dist output.
// index.html contains the app-lang meta placeholder.
func testFS() fs.FS {
	return fstest.MapFS{
		"index.html": {
			Data: []byte(`<!doctype html><html><head>` + langPlaceholder + `</head><body>belochka</body></html>`),
		},
		"favicon.svg":          {Data: []byte("<svg/>")},
		"assets/index-abc.js":  {Data: []byte("console.log('app')")},
		"assets/index-abc.css": {Data: []byte("body{}")},
	}
}

// mockLangStore is an in-memory LangStore for testing.
type mockLangStore struct {
	mu   sync.Mutex
	lang string
}

func (m *mockLangStore) Language() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lang
}

func (m *mockLangStore) SetLanguage(lang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lang = lang
	return nil
}

func TestServesIndexHTML(t *testing.T) {
	h := static.NewHandler(testFS(), nil)

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
	h := static.NewHandler(testFS(), nil)

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
	h := static.NewHandler(testFS(), nil)

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
	h := static.NewHandler(testFS(), nil)

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
	h := static.NewHandler(nil, nil)
	if h != nil {
		t.Fatal("expected nil handler for nil FS")
	}
}

// TestInjectsLanguageIntoMetaTag verifies that a saved language is injected
// into the meta placeholder when serving index.html.
func TestInjectsLanguageIntoMetaTag(t *testing.T) {
	store := &mockLangStore{lang: "zh"}
	h := static.NewHandler(testFS(), store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	want := `<meta name="app-lang" content="zh">`
	if !strings.Contains(body, want) {
		t.Errorf("expected meta tag %q in response, got:\n%s", want, body)
	}
	if strings.Contains(body, langPlaceholder) {
		t.Errorf("placeholder %q should have been replaced, but still present in:\n%s", langPlaceholder, body)
	}
}

// TestInjectsLanguageIntoSPAFallback verifies that language is injected
// even when serving index.html as a SPA fallback.
func TestInjectsLanguageIntoSPAFallback(t *testing.T) {
	store := &mockLangStore{lang: "fr"}
	h := static.NewHandler(testFS(), store)

	req := httptest.NewRequest(http.MethodGet, "/server/some-uuid", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	want := `<meta name="app-lang" content="fr">`
	if !strings.Contains(body, want) {
		t.Errorf("expected meta tag %q in SPA fallback response, got:\n%s", want, body)
	}
}

// TestFirstVisitDetectsLanguageFromAcceptHeader verifies that when no language
// is saved, the server detects it from the Accept-Language header, persists it,
// and injects it into the response.
func TestFirstVisitDetectsLanguageFromAcceptHeader(t *testing.T) {
	store := &mockLangStore{lang: ""} // no saved language
	h := static.NewHandler(testFS(), store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Language should be detected ("zh"), persisted, and injected.
	if store.lang != "zh" {
		t.Errorf("expected persisted language %q, got %q", "zh", store.lang)
	}
	want := `<meta name="app-lang" content="zh">`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("expected meta tag %q in response, got:\n%s", want, rec.Body.String())
	}
}

// TestFirstVisitFallsBackToEnForUnrecognisedLanguage verifies that an
// Accept-Language value with no supported language falls back to "en".
func TestFirstVisitFallsBackToEnForUnrecognisedLanguage(t *testing.T) {
	store := &mockLangStore{lang: ""}
	h := static.NewHandler(testFS(), store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "ja,ko;q=0.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if store.lang != "en" {
		t.Errorf("expected fallback language %q, got %q", "en", store.lang)
	}
	want := `<meta name="app-lang" content="en">`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("expected meta tag %q in response, got:\n%s", want, rec.Body.String())
	}
}

// TestSubsequentVisitsUseSavedLanguage verifies that Accept-Language is ignored
// when a language is already saved.
func TestSubsequentVisitsUseSavedLanguage(t *testing.T) {
	store := &mockLangStore{lang: "ru"}
	h := static.NewHandler(testFS(), store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9") // would detect "zh" but saved is "ru"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Stored language must remain "ru".
	if store.lang != "ru" {
		t.Errorf("expected saved language to stay %q, got %q", "ru", store.lang)
	}
	want := `<meta name="app-lang" content="ru">`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("expected meta tag %q in response, got:\n%s", want, rec.Body.String())
	}
}
