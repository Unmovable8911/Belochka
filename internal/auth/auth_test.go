package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"belochka/internal/clock"
	"belochka/internal/config"
	"belochka/internal/httpx"
)

// fakePasswordStore implements config.ConfigStore for testing.
type fakePasswordStore struct {
	hash     string
	language string
}

func (f *fakePasswordStore) Get() config.Config {
	return config.Config{PasswordHash: f.hash, Language: f.language}
}
func (f *fakePasswordStore) PasswordHash() string { return f.hash }
func (f *fakePasswordStore) Language() string     { return f.language }
func (f *fakePasswordStore) BaseDir() string      { return "" }
func (f *fakePasswordStore) Update(fn func(*config.Config)) error {
	var cfg config.Config
	cfg.PasswordHash = f.hash
	cfg.Language = f.language
	fn(&cfg)
	f.hash = cfg.PasswordHash
	f.language = cfg.Language
	return nil
}

func TestNeedsSetup(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	if !store.NeedsSetup() {
		t.Error("expected NeedsSetup=true when password is empty")
	}

	ps.hash = "$2a$10$something"
	if store.NeedsSetup() {
		t.Error("expected NeedsSetup=false when password is set")
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !VerifyPassword(hash, "secret123") {
		t.Error("VerifyPassword should return true for correct password")
	}
	if VerifyPassword(hash, "wrong") {
		t.Error("VerifyPassword should return false for wrong password")
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
}

func TestHashPasswordTooShort(t *testing.T) {
	_, err := HashPassword("12345")
	if err == nil {
		t.Error("expected error for password < 6 characters")
	}
}

func TestSetupPassword(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	err := store.SetupPassword("en", "secret123", "secret123")
	if err != nil {
		t.Fatalf("SetupPassword: %v", err)
	}
	if ps.hash == "" {
		t.Error("password hash should be set after setup")
	}
	if !VerifyPassword(ps.hash, "secret123") {
		t.Error("password should verify after setup")
	}
	if ps.language != "en" {
		t.Errorf("language should be 'en', got %q", ps.language)
	}
}

func TestSetupPasswordMismatch(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	err := store.SetupPassword("en", "secret123", "different")
	if err == nil || err.Error() != "passwords do not match" {
		t.Errorf("expected 'passwords do not match', got: %v", err)
	}
}

func TestSetupPasswordAlreadySet(t *testing.T) {
	ps := &fakePasswordStore{hash: "somehash"}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	err := store.SetupPassword("en", "secret123", "secret123")
	if err == nil || err.Error() != "password is already set" {
		t.Errorf("expected 'password is already set', got: %v", err)
	}
}

func TestSetupPasswordInvalidLanguage(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	err := store.SetupPassword("ja", "secret123", "secret123")
	if err == nil || err.Error() != "unsupported language: ja" {
		t.Errorf("expected 'unsupported language: ja', got: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	hash, _ := HashPassword("oldpwd")
	ps := &fakePasswordStore{hash: hash}
	fakeClock := clock.NewFake(time.Now())
	store := NewSessionStore(ps, fakeClock)

	// Create a session first.
	w := httptest.NewRecorder()
	_, err := store.CreateSession(w)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(store.sessions) != 1 {
		t.Error("expected 1 active session")
	}

	// Change password.
	err = store.ChangePassword("oldpwd", "newpwd", "newpwd")
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if !VerifyPassword(ps.hash, "newpwd") {
		t.Error("new password should verify")
	}
	if len(store.sessions) != 0 {
		t.Error("all sessions should be cleared after password change")
	}
}

func TestChangePasswordWrongOld(t *testing.T) {
	hash, _ := HashPassword("correct")
	ps := &fakePasswordStore{hash: hash}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	err := store.ChangePassword("wrong", "newpwd", "newpwd")
	if err == nil || err.Error() != "current password is incorrect" {
		t.Errorf("expected 'current password is incorrect', got: %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	ps := &fakePasswordStore{hash: "somehash"}
	fakeClock := clock.NewFake(time.Now())
	store := NewSessionStore(ps, fakeClock)

	ip := "192.168.1.1"

	// Should start clean.
	if err := store.CheckRateLimit(ip); err != nil {
		t.Errorf("expected no rate limit, got: %v", err)
	}

	// Record failures up to the limit.
	for i := 0; i < maxFailedAttempts-1; i++ {
		remaining := store.RecordFailedAttempt(ip)
		if remaining != 0 {
			t.Errorf("expected no lockout at attempt %d, got remaining=%v", i+1, remaining)
		}
	}

	// Should still be allowed.
	if err := store.CheckRateLimit(ip); err != nil {
		t.Errorf("expected no rate limit before threshold, got: %v", err)
	}

	// 10th failure should trigger lockout.
	remaining := store.RecordFailedAttempt(ip)
	if remaining != lockoutDuration {
		t.Errorf("expected lockout duration %v, got %v", lockoutDuration, remaining)
	}

	// Should now be locked.
	if err := store.CheckRateLimit(ip); err == nil {
		t.Error("expected rate limit error after threshold")
	}

	// Clear resets.
	store.ClearRateLimit(ip)
	if err := store.CheckRateLimit(ip); err != nil {
		t.Errorf("expected no rate limit after clear, got: %v", err)
	}
}

func TestRateLimitExpiry(t *testing.T) {
	ps := &fakePasswordStore{hash: "somehash"}
	fakeClock := clock.NewFake(time.Now())
	store := NewSessionStore(ps, fakeClock)

	ip := "10.0.0.1"

	// Trigger lockout.
	for i := 0; i < maxFailedAttempts; i++ {
		store.RecordFailedAttempt(ip)
	}

	// Advance past lockout.
	fakeClock.Advance(lockoutDuration + time.Second)

	if err := store.CheckRateLimit(ip); err != nil {
		t.Errorf("expected rate limit to expire, got: %v", err)
	}
}

func TestCreateAndValidateSession(t *testing.T) {
	ps := &fakePasswordStore{}
	fakeClock := clock.NewFake(time.Now())
	store := NewSessionStore(ps, fakeClock)

	w := httptest.NewRecorder()
	_, err := store.CreateSession(w)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Extract cookie from response.
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	// Build a request with the cookie.
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	if !store.ValidateSession(req) {
		t.Error("expected session to be valid")
	}
}

func TestValidateSessionExpired(t *testing.T) {
	ps := &fakePasswordStore{}
	fakeClock := clock.NewFake(time.Now())
	store := NewSessionStore(ps, fakeClock)

	w := httptest.NewRecorder()
	_, err := store.CreateSession(w)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	cookies := w.Result().Cookies()

	// Advance past session expiry.
	fakeClock.Advance(sessionDuration + time.Second)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	if store.ValidateSession(req) {
		t.Error("expected expired session to be invalid")
	}
}

func TestValidateSessionNoCookie(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	req := httptest.NewRequest("GET", "/", nil)
	if store.ValidateSession(req) {
		t.Error("expected no session when cookie is missing")
	}
}

func TestDeleteSession(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	w := httptest.NewRecorder()
	_, err := store.CreateSession(w)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	cookies := w.Result().Cookies()

	// Delete the session.
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(cookies[0])
	store.DeleteSession(w2, req)

	// Session should be invalid now.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(cookies[0])
	if store.ValidateSession(req2) {
		t.Error("session should be invalid after deletion")
	}
}

func TestDeleteAllSessions(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	// Create two sessions.
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		store.CreateSession(w)
	}

	if len(store.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(store.sessions))
	}

	store.DeleteAllSessions()
	if len(store.sessions) != 0 {
		t.Error("expected 0 sessions after DeleteAllSessions")
	}
}

func TestCleanupExpired(t *testing.T) {
	ps := &fakePasswordStore{}
	fakeClock := clock.NewFake(time.Now())
	store := NewSessionStore(ps, fakeClock)

	w := httptest.NewRecorder()
	store.CreateSession(w)

	// Advance past expiry.
	fakeClock.Advance(sessionDuration + time.Second)

	store.CleanupExpired()
	if len(store.sessions) != 0 {
		t.Error("expected expired sessions to be cleaned up")
	}
}

// --- Middleware tests ---

func TestMiddlewareWithoutSession(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/protected", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMiddlewareWithValidSession(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))

	// Create a session.
	w1 := httptest.NewRecorder()
	store.CreateSession(w1)
	cookies := w1.Result().Cookies()

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Context().Value(AuthenticatedKey); v != true {
			t.Error("expected authenticated context key to be true")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.AddCookie(cookies[0])
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- Handler integration tests ---

func setupHandler() (*Handler, *fakePasswordStore) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))
	return NewHandler(store), ps
}

func postJSON(url string, body string) *http.Request {
	req := httptest.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func TestHandleSetup(t *testing.T) {
	h, ps := setupHandler()

	// Successful setup.
	w := httptest.NewRecorder()
	h.HandleSetup(w, postJSON("/api/setup", `{"language":"en","password":"secret123","confirm_password":"secret123"}`))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ps.hash == "" {
		t.Error("password hash should be set")
	}
	if ps.language != "en" {
		t.Errorf("expected language 'en', got %q", ps.language)
	}

	// Setup again should fail.
	w2 := httptest.NewRecorder()
	h.HandleSetup(w2, postJSON("/api/setup", `{"language":"en","password":"another","confirm_password":"another"}`))
	if w2.Code != http.StatusForbidden {
		t.Errorf("expected 403 for double setup, got %d", w2.Code)
	}
}

func TestHandleSetupMismatch(t *testing.T) {
	h, _ := setupHandler()

	w := httptest.NewRecorder()
	h.HandleSetup(w, postJSON("/api/setup", `{"language":"en","password":"secret123","confirm_password":"different"}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetupMissingLanguage(t *testing.T) {
	h, _ := setupHandler()

	w := httptest.NewRecorder()
	h.HandleSetup(w, postJSON("/api/setup", `{"password":"secret123","confirm_password":"secret123"}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing language, got %d", w.Code)
	}
}

func TestHandleSetupInvalidLanguage(t *testing.T) {
	h, _ := setupHandler()

	w := httptest.NewRecorder()
	h.HandleSetup(w, postJSON("/api/setup", `{"language":"ja","password":"secret123","confirm_password":"secret123"}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid language, got %d", w.Code)
	}
}

func TestHandleLogin(t *testing.T) {
	h, ps := setupHandler()

	// Setup first.
	ps.hash, _ = HashPassword("testpwd")

	// Successful login.
	w := httptest.NewRecorder()
	h.HandleLogin(w, postJSON("/api/login", `{"password":"testpwd"}`))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check that session cookie is set.
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("session cookie should be set after login")
	}
}

func TestHandleLoginInvalidPassword(t *testing.T) {
	h, ps := setupHandler()
	ps.hash, _ = HashPassword("correct")

	w := httptest.NewRecorder()
	h.HandleLogin(w, postJSON("/api/login", `{"password":"wrong"}`))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleLoginNoSetup(t *testing.T) {
	h, _ := setupHandler()

	w := httptest.NewRecorder()
	h.HandleLogin(w, postJSON("/api/login", `{"password":"anything"}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleAuthStatus(t *testing.T) {
	h, ps := setupHandler()

	// Not set up, not authenticated.
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	h.HandleAuthStatus(w, req)

	var resp authStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.NeedsSetup {
		t.Error("expected needs_setup=true")
	}
	if resp.Authenticated {
		t.Error("expected authenticated=false")
	}

	// Set password but not logged in.
	ps.hash, _ = HashPassword("test123")

	w2 := httptest.NewRecorder()
	h.HandleAuthStatus(w2, httptest.NewRequest("GET", "/api/auth/status", nil))
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp.NeedsSetup {
		t.Error("expected needs_setup=false after password set")
	}
}

func TestHandleLogout(t *testing.T) {
	h, ps := setupHandler()
	ps.hash, _ = HashPassword("test123")

	// Login first to get a session.
	w1 := httptest.NewRecorder()
	h.HandleLogin(w1, postJSON("/api/login", `{"password":"test123"}`))
	cookies := w1.Result().Cookies()

	// Logout with session cookie.
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	h.HandleLogout(w2, req)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}

	// Cookie should be cleared.
	respCookies := w2.Result().Cookies()
	found := false
	for _, c := range respCookies {
		if c.Name == sessionCookieName && c.MaxAge < 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("session cookie should be cleared on logout")
	}
}

func TestHandleChangePassword(t *testing.T) {
	h, ps := setupHandler()
	ps.hash, _ = HashPassword("oldpwd")

	// Login first.
	w1 := httptest.NewRecorder()
	h.HandleLogin(w1, postJSON("/api/login", `{"password":"oldpwd"}`))

	w := httptest.NewRecorder()
	h.HandleChangePassword(w, postJSON("/api/change-password", `{"old_password":"oldpwd","new_password":"newpwd","confirm_password":"newpwd"}`))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !VerifyPassword(ps.hash, "newpwd") {
		t.Error("password should be changed")
	}

	// New session should be set.
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("new session cookie should be set after password change")
	}
}

func TestHandleChangePasswordWrongOld(t *testing.T) {
	h, ps := setupHandler()
	ps.hash, _ = HashPassword("oldpwd")

	w := httptest.NewRecorder()
	h.HandleChangePassword(w, postJSON("/api/change-password", `{"old_password":"wrong","new_password":"newpwd","confirm_password":"newpwd"}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"simple", "192.168.1.1:12345", "", "192.168.1.1"},
		{"xff single", "10.0.0.1:443", "203.0.113.1", "203.0.113.1"},
		{"xff multiple", "10.0.0.1:443", "203.0.113.1, 10.0.0.2", "203.0.113.1"},
		{"no port", "192.168.1.1", "", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(req); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFullFlowIntegration tests the complete auth lifecycle via HTTP.
func TestFullFlowIntegration(t *testing.T) {
	ps := &fakePasswordStore{}
	store := NewSessionStore(ps, clock.NewFake(time.Now()))
	handler := NewHandler(store)

	// 1. Check auth status — needs setup.
	checkStatus := func(wantSetup, wantAuth bool) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.HandleAuthStatus(w, httptest.NewRequest("GET", "/api/auth/status", nil))
		var s authStatusResponse
		json.NewDecoder(w.Body).Decode(&s)
		if s.NeedsSetup != wantSetup {
			t.Errorf("needs_setup: want %v, got %v", wantSetup, s.NeedsSetup)
		}
		if s.Authenticated != wantAuth {
			t.Errorf("authenticated: want %v, got %v", wantAuth, s.Authenticated)
		}
	}
	checkStatus(true, false)

	// 2. Setup password.
	w := httptest.NewRecorder()
	handler.HandleSetup(w, postJSON("/api/setup", `{"language":"zh","password":"mypassword","confirm_password":"mypassword"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d", w.Code)
	}
	cookies := w.Result().Cookies()
	checkStatus(false, false) // Not authenticated yet (cookie not sent).

	// 3. Verify auth status with cookie.
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	handler.HandleAuthStatus(w2, req)
	var s authStatusResponse
	json.NewDecoder(w2.Body).Decode(&s)
	if !s.Authenticated {
		t.Error("should be authenticated with session cookie")
	}

	// 4. Logout.
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("POST", "/api/logout", nil)
	for _, c := range cookies {
		req3.AddCookie(c)
	}
	handler.HandleLogout(w3, req3)

	// 5. Verify not authenticated after logout.
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest("GET", "/api/auth/status", nil)
	handler.HandleAuthStatus(w4, req4)
	json.NewDecoder(w4.Body).Decode(&s)
	if s.Authenticated {
		t.Error("should not be authenticated after logout")
	}

	// 6. Login.
	w5 := httptest.NewRecorder()
	handler.HandleLogin(w5, postJSON("/api/login", `{"password":"mypassword"}`))
	if w5.Code != http.StatusOK {
		t.Fatalf("login: %d — %s", w5.Code, w5.Body.String())
	}

	// 7. Change password.
	loginCookies := w5.Result().Cookies()
	w6 := httptest.NewRecorder()
	req6 := postJSON("/api/change-password", `{"old_password":"mypassword","new_password":"newpassword","confirm_password":"newpassword"}`)
	for _, c := range loginCookies {
		req6.AddCookie(c)
	}
	handler.HandleChangePassword(w6, req6)
	if w6.Code != http.StatusOK {
		t.Fatalf("change password: %d — %s", w6.Code, w6.Body.String())
	}

	// 8. Login with new password.
	w7 := httptest.NewRecorder()
	handler.HandleLogin(w7, postJSON("/api/login", `{"password":"newpassword"}`))
	if w7.Code != http.StatusOK {
		t.Errorf("login with new password: %d — %s", w7.Code, w7.Body.String())
	}
}

func TestRateLimitIntegration(t *testing.T) {
	ps := &fakePasswordStore{hash: ""}
	hash, _ := HashPassword("correct")
	ps.hash = hash
	store := NewSessionStore(ps, clock.NewFake(time.Now()))
	handler := NewHandler(store)

	// Send many wrong password attempts.
	for i := 0; i < maxFailedAttempts; i++ {
		w := httptest.NewRecorder()
		req := postJSON("/api/login", `{"password":"wrong"}`)
		req.RemoteAddr = "10.0.0.99:12345"
		handler.HandleLogin(w, req)
	}

	// Next attempt should be rate-limited.
	w := httptest.NewRecorder()
	req := postJSON("/api/login", `{"password":"correct"}`)
	req.RemoteAddr = "10.0.0.99:12345"
	handler.HandleLogin(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	// Verify response body contains rate_limited error.
	body, _ := io.ReadAll(w.Body)
	var resp httpx.ErrorBody
	json.Unmarshal(body, &resp)
	if resp.Error.Code != "rate_limited" {
		t.Errorf("expected rate_limited code, got %q", resp.Error.Code)
	}
}
