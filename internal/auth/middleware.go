package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

// AuthenticatedKey is the context key for the authenticated flag.
const AuthenticatedKey contextKey = "authenticated"

// Middleware returns a chi-compatible middleware that validates the session
// cookie. Requests without a valid session receive a 401 JSON response.
func Middleware(store *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !store.ValidateSession(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "unauthorized",
				})
				return
			}
			ctx := context.WithValue(r.Context(), AuthenticatedKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
