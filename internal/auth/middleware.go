package auth

import (
	"context"
	"net/http"

	"belochka/internal/httpx"
)

type contextKey string

// AuthenticatedKey is the context key for the authenticated flag.
const AuthenticatedKey contextKey = "authenticated"

// Middleware returns a chi-compatible middleware that validates the session
// cookie. Requests without a valid session receive a 401 JSON response.
// When store is nil (auth not configured), it passes through without validation.
func Middleware(store *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !store.ValidateSession(r) {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
				return
			}
			ctx := context.WithValue(r.Context(), AuthenticatedKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
