package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"belochka/internal/clock"

	"belochka/internal/config"

	"golang.org/x/crypto/bcrypt"
)

// Sentinal errors returned by ChangePassword.
var (
	ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
	ErrPasswordsDoNotMatch      = errors.New("passwords do not match")
	ErrPasswordTooShort         = errors.New("password too short")
)

const (
	sessionCookieName = "belochka_session"
	sessionDuration   = 7 * 24 * time.Hour
	maxFailedAttempts = 10
	lockoutDuration   = 30 * time.Minute
	minPasswordLength = 6
)

type session struct {
	id        string
	createdAt time.Time
	expiresAt time.Time
}

type rateLimitState struct {
	failures    int
	lockedUntil time.Time
}

// SessionStore manages authentication state: sessions, password hashing,
// and login rate limiting.
type SessionStore struct {
	mu            sync.RWMutex
	sessions      map[string]*session
	passwordStore config.ConfigStore
	rateLimits    map[string]*rateLimitState
	clock         clock.Clock
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(ps config.ConfigStore, c clock.Clock) *SessionStore {
	return &SessionStore{
		sessions:      make(map[string]*session),
		passwordStore: ps,
		rateLimits:    make(map[string]*rateLimitState),
		clock:         c,
	}
}

// NeedsSetup returns true when no password has been set.
func (s *SessionStore) NeedsSetup() bool {
	return s.passwordStore.PasswordHash() == ""
}

// HashPassword returns a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	if len(password) < minPasswordLength {
		return "", fmt.Errorf("%w: must be at least %d characters", ErrPasswordTooShort, minPasswordLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword compares a password against a bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SetupPassword hashes and persists the initial password and language.
// Returns an error if a password is already set, the password is too short,
// or the language is not supported.
func (s *SessionStore) SetupPassword(language, password, confirm string) error {
	if !s.NeedsSetup() {
		return fmt.Errorf("password is already set")
	}
	if !isSupportedLanguage(language) {
		return fmt.Errorf("unsupported language: %s", language)
	}
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.passwordStore.Update(func(c *config.Config) {
		c.PasswordHash = hash
		c.Language = language
	})
}

// supportedLanguages is the set of languages the UI supports.
var supportedLanguages = map[string]bool{
	"en":    true,
	"zh":    true,
	"fr":    true,
	"ru":    true,
	"de":    true,
	"es":    true,
	"pt":    true,
	"zh-TW": true,
	"it":    true,
}

func isSupportedLanguage(lang string) bool {
	return supportedLanguages[lang]
}

// ChangePassword verifies the old password, hashes and persists the new one,
// and clears all existing sessions.
func (s *SessionStore) ChangePassword(oldPassword, newPassword, confirm string) error {
	if s.NeedsSetup() {
		return fmt.Errorf("no password is set")
	}
	if !VerifyPassword(s.passwordStore.PasswordHash(), oldPassword) {
		return fmt.Errorf("%w", ErrCurrentPasswordIncorrect)
	}
	if newPassword != confirm {
		return fmt.Errorf("%w", ErrPasswordsDoNotMatch)
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.passwordStore.Update(func(c *config.Config) { c.PasswordHash = hash }); err != nil {
		return err
	}
	s.DeleteAllSessions()
	return nil
}

// CheckRateLimit returns an error if the given IP is currently locked out.
func (s *SessionStore) CheckRateLimit(ip string) error {
	s.mu.RLock()
	state, ok := s.rateLimits[ip]
	s.mu.RUnlock()

	if !ok || state.lockedUntil.IsZero() {
		return nil
	}

	now := s.clock.Now()
	if now.Before(state.lockedUntil) {
		remaining := state.lockedUntil.Sub(now).Round(time.Second)
		return fmt.Errorf("too many failed attempts; try again in %s", remaining)
	}

	// Lockout expired.
	return nil
}

// RecordFailedAttempt increments the failure counter for an IP and locks
// the IP out after maxFailedAttempts.
func (s *SessionStore) RecordFailedAttempt(ip string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()
	state, ok := s.rateLimits[ip]
	if !ok {
		state = &rateLimitState{}
		s.rateLimits[ip] = state
	} else if !state.lockedUntil.IsZero() && !now.Before(state.lockedUntil) {
		// Previous lockout expired — reset.
		state = &rateLimitState{}
		s.rateLimits[ip] = state
	}

	// If already locked, don't increment further.
	if !state.lockedUntil.IsZero() && now.Before(state.lockedUntil) {
		return state.lockedUntil.Sub(now)
	}

	state.failures++
	if state.failures >= maxFailedAttempts {
		state.lockedUntil = now.Add(lockoutDuration)
		return lockoutDuration
	}
	return 0
}

// ClearRateLimit removes rate-limit state for an IP (called on successful login).
func (s *SessionStore) ClearRateLimit(ip string) {
	s.mu.Lock()
	delete(s.rateLimits, ip)
	s.mu.Unlock()
}

// CreateSession generates a new session, stores it, and sets the cookie on w.
func (s *SessionStore) CreateSession(w http.ResponseWriter) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	now := s.clock.Now()
	sess := &session{
		id:        id,
		createdAt: now,
		expiresAt: now.Add(sessionDuration),
	}

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	setSessionCookie(w, id, sessionDuration)
	return id, nil
}

// ValidateSession checks the request cookie and returns whether the session is valid.
func (s *SessionStore) ValidateSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}

	s.mu.RLock()
	sess, ok := s.sessions[cookie.Value]
	s.mu.RUnlock()

	if !ok {
		return false
	}

	// Constant-time comparison to prevent timing leaks on the session ID.
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sess.id)) != 1 {
		return false
	}

	now := s.clock.Now()
	if now.After(sess.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, sess.id)
		s.mu.Unlock()
		return false
	}

	return true
}

// DeleteSession removes a session and clears its cookie.
func (s *SessionStore) DeleteSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}

	s.mu.Lock()
	delete(s.sessions, cookie.Value)
	s.mu.Unlock()

	clearSessionCookie(w)
}

// DeleteAllSessions removes every active session (used on password change).
func (s *SessionStore) DeleteAllSessions() {
	s.mu.Lock()
	s.sessions = make(map[string]*session)
	s.mu.Unlock()
}

// CleanupExpired removes sessions past their expiry. Designed to be called
// periodically from a background goroutine.
func (s *SessionStore) CleanupExpired() {
	now := s.clock.Now()
	s.mu.Lock()
	for id, sess := range s.sessions {
		if now.After(sess.expiresAt) {
			delete(s.sessions, id)
		}
	}
	// Also clean up expired rate-limit entries.
	for ip, state := range s.rateLimits {
		if !now.Before(state.lockedUntil) && state.failures < maxFailedAttempts {
			delete(s.rateLimits, ip)
		}
	}
	s.mu.Unlock()
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func setSessionCookie(w http.ResponseWriter, sessionID string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
