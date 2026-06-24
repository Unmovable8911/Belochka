package ssh

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"belochka/internal/clock"
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second

	// KeepaliveInterval is the interval between SSH keepalive pings.
	KeepaliveInterval = 30 * time.Second

	// KeepaliveFailureThreshold is the number of consecutive keepalive
	// failures that mark a connection as dead.
	KeepaliveFailureThreshold = 3

	// CollectionFailureThreshold is the number of consecutive collection
	// failures that trigger SSH reconnection.
	CollectionFailureThreshold = 3
)

// ConnState represents the state of a reconnecting connection.
type ConnState string

const (
	StateConnected    ConnState = "connected"
	StateReconnecting ConnState = "reconnecting"
	StateFailed       ConnState = "failed" // non-retryable error; reconnection stopped
)

// ConnStatus holds the current state of a Reconnector, suitable for display.
type ConnStatus struct {
	State     ConnState `json:"state"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"last_error,omitempty"`
}

// ConnectFunc is the function called to establish (or re-establish) a connection.
// It should block until the connection is established or an error occurs.
type ConnectFunc func(ctx context.Context) error

// IsRetryable reports whether the error is retryable for reconnection purposes.
// Auth failures and host key mismatches are non-retryable.
// All other errors (network, timeout, unknown) are retryable.
func IsRetryable(err error) bool {
	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		switch connErr.Kind {
		case ErrAuth, ErrHostKey:
			return false
		}
	}
	return true
}

// backoffDelay returns the backoff duration for the given attempt number (0-based).
// Schedule: 1s, 2s, 4s, 8s, 16s, 30s (capped).
func backoffDelay(attempt int) time.Duration {
	d := initialBackoff
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}

// Reconnector manages automatic reconnection with exponential backoff.
// It calls a ConnectFunc repeatedly until it succeeds, encounters a
// non-retryable error, or the context is cancelled.
type Reconnector struct {
	connect ConnectFunc
	clock   clock.Clock

	mu        sync.RWMutex
	state     ConnState
	attempts  int
	lastError string
}

// NewReconnector creates a new Reconnector with the given connect function.
func NewReconnector(connect ConnectFunc) *Reconnector {
	return &Reconnector{
		connect: connect,
		state:   StateReconnecting,
		clock:   clock.Real{},
	}
}

// Run starts the reconnection loop. It blocks until one of:
//   - The connection succeeds (state becomes Connected)
//   - A non-retryable error is encountered (state becomes Failed)
//   - The context is cancelled
//
// After a successful connection, Run returns. The caller is responsible
// for calling Run again if the connection drops later.
func (r *Reconnector) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		r.mu.Lock()
		attempt := r.attempts
		r.state = StateReconnecting
		r.mu.Unlock()

		err := r.connect(ctx)
		if err == nil {
			// Success
			r.mu.Lock()
			r.state = StateConnected
			r.attempts = 0
			r.lastError = ""
			r.mu.Unlock()
			slog.Info("SSH reconnection successful", "after_attempts", attempt)
			return
		}

		r.mu.Lock()
		r.attempts++
		r.lastError = err.Error()
		currentAttempt := r.attempts
		r.mu.Unlock()

		if !IsRetryable(err) {
			r.mu.Lock()
			r.state = StateFailed
			r.mu.Unlock()
			slog.Warn("SSH reconnection stopped: non-retryable error",
				"error", err,
				"attempts", currentAttempt,
			)
			return
		}

		slog.Info("SSH reconnection attempt failed, will retry",
			"attempt", currentAttempt,
			"error", err,
			"next_backoff", backoffDelay(currentAttempt-1),
		)

		delay := backoffDelay(currentAttempt - 1)
		if err := r.clock.Sleep(ctx, delay); err != nil {
			return // context cancelled
		}
	}
}

// Status returns the current reconnection status.
func (r *Reconnector) Status() ConnStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return ConnStatus{
		State:     r.state,
		Attempts:  r.attempts,
		LastError: r.lastError,
	}
}

// Reset resets the reconnector state back to reconnecting with 0 attempts.
// Call this before starting a new Run cycle.
func (r *Reconnector) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = StateReconnecting
	r.attempts = 0
	r.lastError = ""
}

// PingFunc sends a keepalive ping and returns an error if it fails.
type PingFunc func(ctx context.Context) error

// Keepalive sends periodic keepalive pings and triggers reconnection
// after KeepaliveFailureThreshold consecutive failures.
type Keepalive struct {
	ping        PingFunc
	onReconnect func()
	interval    time.Duration
	clock       clock.Clock
}

// NewKeepalive creates a new Keepalive monitor.
func NewKeepalive(ping PingFunc, onReconnect func()) *Keepalive {
	return &Keepalive{
		ping:        ping,
		onReconnect: onReconnect,
		interval:    KeepaliveInterval,
		clock:       clock.Real{},
	}
}

// Run starts the keepalive loop. It blocks until ctx is cancelled.
// When KeepaliveFailureThreshold consecutive pings fail, it calls
// onReconnect and resets the counter.
func (k *Keepalive) Run(ctx context.Context) {
	ticker := k.clock.NewTicker(k.interval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			if err := k.ping(ctx); err != nil {
				consecutiveFailures++
				slog.Debug("keepalive ping failed",
					"failures", consecutiveFailures,
					"error", err,
				)
				if consecutiveFailures >= KeepaliveFailureThreshold {
					slog.Warn("keepalive: connection dead, triggering reconnection",
						"consecutive_failures", consecutiveFailures,
					)
					k.onReconnect()
					consecutiveFailures = 0
				}
			} else {
				consecutiveFailures = 0
			}
		}
	}
}
