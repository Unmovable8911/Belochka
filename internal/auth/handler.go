package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Handler holds HTTP handlers for authentication endpoints.
type Handler struct {
	store *SessionStore
}

// NewHandler creates a new Handler.
func NewHandler(store *SessionStore) *Handler {
	return &Handler{store: store}
}

type setupRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type loginRequest struct {
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword     string `json:"old_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type authStatusResponse struct {
	NeedsSetup    bool `json:"needs_setup"`
	Authenticated bool `json:"authenticated"`
}

// HandleSetup processes POST /api/setup. Only works when no password is set.
func (h *Handler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	if !h.store.NeedsSetup() {
		writeJSON(w, http.StatusForbidden, errBody("already_setup", "Password is already set"))
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_json", "Request body is not valid JSON"))
		return
	}

	if err := h.store.SetupPassword(req.Password, req.ConfirmPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_input", err.Error()))
		return
	}

	if _, err := h.store.CreateSession(w); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal_error", "Failed to create session"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleLogin processes POST /api/login.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	if err := h.store.CheckRateLimit(ip); err != nil {
		writeJSON(w, http.StatusTooManyRequests, errBody("rate_limited", err.Error()))
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_json", "Request body is not valid JSON"))
		return
	}

	if h.store.NeedsSetup() {
		writeJSON(w, http.StatusBadRequest, errBody("setup_required", "No password is set. Visit /setup first."))
		return
	}

	if !VerifyPassword(h.store.passwordStore.PasswordHash(), req.Password) {
		if remaining := h.store.RecordFailedAttempt(ip); remaining > 0 {
			writeJSON(w, http.StatusTooManyRequests, errBody("rate_limited", "Too many failed attempts; try again later"))
			return
		}
		writeJSON(w, http.StatusUnauthorized, errBody("invalid_password", "Invalid password"))
		return
	}

	h.store.ClearRateLimit(ip)

	if _, err := h.store.CreateSession(w); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal_error", "Failed to create session"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleLogout processes POST /api/logout.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	h.store.DeleteSession(w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleAuthStatus processes GET /api/auth/status.
func (h *Handler) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, authStatusResponse{
		NeedsSetup:    h.store.NeedsSetup(),
		Authenticated: h.store.ValidateSession(r),
	})
}

// HandleChangePassword processes POST /api/change-password.
func (h *Handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_json", "Request body is not valid JSON"))
		return
	}

	if err := h.store.ChangePassword(req.OldPassword, req.NewPassword, req.ConfirmPassword); err != nil {
		code := "invalid_input"
		switch {
		case errors.Is(err, ErrCurrentPasswordIncorrect):
			code = "incorrect_password"
		case errors.Is(err, ErrPasswordsDoNotMatch):
			code = "passwords_dont_match"
		case errors.Is(err, ErrPasswordTooShort):
			code = "password_too_short"
		}
		writeJSON(w, http.StatusBadRequest, errBody(code, err.Error()))
		return
	}

	// Create a new session so the user stays logged in after changing password.
	if _, err := h.store.CreateSession(w); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal_error", "Failed to create session"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// clientIP extracts the client IP from the request, respecting X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errBody(code, message string) errorResponse {
	return errorResponse{Error: errorDetail{Code: code, Message: message}}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
