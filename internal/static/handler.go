// Package static serves embedded frontend assets with SPA fallback.
package static

import (
	"io/fs"
	"net/http"
	"strings"

	"belochka/internal/config"
)

// supportedLangs are the UI language codes the app ships with, kept in the
// same (native-name) order as the frontend LANGUAGES list.
var supportedLangs = []string{"de", "en", "es", "fr", "it", "pt", "ru", "zh", "zh-TW"}

// langAliases maps Accept-Language tags that don't match a supported code
// exactly to the closest supported code (e.g. zh-HK is Traditional Chinese).
var langAliases = map[string]string{
	"zh-hant": "zh-TW",
	"zh-hk":   "zh-TW",
	"zh-mo":   "zh-TW",
}

// detectLanguage picks the best supported language from an Accept-Language
// header value. An exact tag match wins, then a known alias, then a match on
// the base language (so zh-CN resolves to zh). Falls back to "en" when no
// match is found.
func detectLanguage(acceptLang string) string {
	for _, part := range strings.Split(acceptLang, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if alias, ok := langAliases[tag]; ok {
			return alias
		}
		for _, lang := range supportedLangs {
			if strings.ToLower(lang) == tag {
				return lang
			}
		}
		base := strings.SplitN(tag, "-", 2)[0]
		for _, lang := range supportedLangs {
			if strings.ToLower(lang) == base {
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
func NewHandler(fsys fs.FS, store config.ConfigStore) http.Handler {
	if fsys == nil {
		return nil
	}
	return &spaHandler{fs: http.FileServerFS(fsys), raw: fsys, store: store}
}

type spaHandler struct {
	fs    http.Handler
	raw   fs.FS
	store config.ConfigStore
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
	// Validate lang against supported languages to prevent XSS via config injection.
	safeLang := "en"
	for _, l := range supportedLangs {
		if l == lang {
			safeLang = lang
			break
		}
	}

	body := strings.Replace(
		string(data),
		placeholder,
		`<meta name="app-lang" content="`+safeLang+`">`,
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
	_ = h.store.Update(func(c *config.Config) { c.Language = lang }) // best-effort; ignore error
	return lang
}
