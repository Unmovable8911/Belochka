package wsutil

import (
	"net/http"
	"strings"
)

// CheckOrigin allows same-origin and localhost WebSocket upgrade requests.
// This is a localhost-first tool; cross-origin access is not intended.
func CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser clients (curl, wscat)
	}
	host := r.Host
	return strings.HasPrefix(origin, "http://"+host) ||
		strings.HasPrefix(origin, "https://"+host) ||
		strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "https://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1") ||
		strings.HasPrefix(origin, "https://127.0.0.1")
}
