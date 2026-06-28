// Package static serves embedded frontend assets with SPA fallback.
package static

import (
	"io/fs"
	"net/http"
	"strings"
)

// LangStore provides read/write access to the persisted UI language.
type LangStore interface {
	Language() string
	SetLanguage(lang string) error
}

// supportedLangs are the UI language codes the app ships with.
var supportedLangs = []string{"en", "zh", "fr", "ru"}

// detectLanguage picks the best supported language from an Accept-Language
// header value. Falls back to "en" when no match is found.
func detectLanguage(acceptLang string) string {
	for _, part := range strings.Split(acceptLang, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		base := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		for _, lang := range supportedLangs {
			if base == lang {
				return lang
			}
		}
	}
	return "en"
}

// NewHandler returns an http.Handler that serves files from the given fs.FS.
// Paths that don't match a file are served index.html (SPA client-side routing).
// When store is non-nil, the handler injects the current language into the
// <meta name="app-lang"> placeholder in index.html on every response; on the
// first visit (empty language) it detects the language from the Accept-Language
// header and persists it via store.
// Returns nil if fsys is nil (development mode — no embedded assets).
func NewHandler(fsys fs.FS, store LangStore) http.Handler {
	if fsys == nil {
		return nil
	}
	return &spaHandler{fs: http.FileServerFS(fsys), raw: fsys, store: store}
}

type spaHandler struct {
	fs    http.Handler
	raw   fs.FS
	store LangStore
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	if path == "index.html" {
		h.serveIndex(w, r)
		return
	}

	// Check if the file exists in the embedded filesystem.
	f, err := h.raw.Open(path)
	if err == nil {
		f.Close()
		// File exists — serve it with the standard file server.
		h.fs.ServeHTTP(w, r)
		return
	}

	// File doesn't exist — serve index.html for SPA routing.
	h.serveIndex(w, r)
}

// serveIndex reads index.html, injects the UI language into the meta tag
// placeholder, and writes the result to w.
func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	lang := h.resolveLanguage(r)

	data, err := fs.ReadFile(h.raw, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}

	const placeholder = `<meta name="app-lang" content="">`
	body := strings.Replace(
		string(data),
		placeholder,
		`<meta name="app-lang" content="`+lang+`">`,
		1,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(body)) //nolint:errcheck
}

// resolveLanguage returns the language to use for this request. If a language
// is already saved in the store it is returned as-is. Otherwise the
// Accept-Language header is parsed, the best match is persisted, and returned.
// When store is nil, returns "en".
func (h *spaHandler) resolveLanguage(r *http.Request) string {
	if h.store == nil {
		return "en"
	}
	lang := h.store.Language()
	if lang != "" {
		return lang
	}
	lang = detectLanguage(r.Header.Get("Accept-Language"))
	_ = h.store.SetLanguage(lang) // best-effort; ignore error
	return lang
}
