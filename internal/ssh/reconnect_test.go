package ssh

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBackoff_exponentialSequence(t *testing.T) {
	// Backoff schedule: 1s, 2s, 4s, 8s, 16s, 30s cap
	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second, // cap
		30 * time.Second, // stays at cap
	}

	for i, want := range expected {
		got := backoffDelay(i)
		if got != want {
			t.Errorf("backoffDelay(%d) = %v, want %v", i, got, want)
		}
	}
}

func TestBackoff_zeroAttemptIsOneSecond(t *testing.T) {
	got := backoffDelay(0)
	if got != 1*time.Second {
		t.Errorf("backoffDelay(0) = %v, want 1s", got)
	}
}

func TestIsRetryable_authFailureIsNotRetryable(t *testing.T) {
	err := &ConnectionError{Kind: ErrAuth, Message: "auth failed"}
	if IsRetryable(err) {
		t.Error("auth failure should not be retryable")
	}
}

func TestIsRetryable_hostKeyMismatchIsNotRetryable(t *testing.T) {
	err := &ConnectionError{Kind: ErrHostKey, Message: "host key mismatch"}
	if IsRetryable(err) {
		t.Error("host key mismatch should not be retryable")
	}
}

func TestIsRetryable_networkErrorIsRetryable(t *testing.T) {
	err := &ConnectionError{Kind: ErrNetwork, Message: "connection refused"}
	if !IsRetryable(err) {
		t.Error("network error should be retryable")
	}
}

func TestIsRetryable_genericErrorIsRetryable(t *testing.T) {
	err := fmt.Errorf("some unknown error")
	if !IsRetryable(err) {
		t.Error("generic error should be retryable (fail-open)")
	}
}

// --- Reconnector tests ---

// fakeConnector tracks connection attempts and can be configured to succeed or fail.
type fakeConnector struct {
	mu       sync.Mutex
	attempts int
	err      error // nil = success
}

func (f *fakeConnector) connect(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	return f.err
}

func (f *fakeConnector) getAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

func (f *fakeConnector) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// delayRecorder records backoff delays requested without actually sleeping.
type delayRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (d *delayRecorder) sleep(ctx context.Context, dur time.Duration) error {
	d.mu.Lock()
	d.delays = append(d.delays, dur)
	d.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func (d *delayRecorder) getDelays() []time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]time.Duration, len(d.delays))
	copy(out, d.delays)
	return out
}

// newFastReconnector creates a Reconnector with instant sleeps for testing.
func newFastReconnector(connect ConnectFunc) (*Reconnector, *delayRecorder) {
	r := NewReconnector(connect)
	dr := &delayRecorder{}
	r.sleepFn = dr.sleep
	return r, dr
}

func TestReconnector_reconnectsOnRetryableError(t *testing.T) {
	fc := &fakeConnector{err: &ConnectionError{Kind: ErrNetwork, Message: "connection refused"}}

	r, _ := newFastReconnector(fc.connect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx)

	// Wait for at least 2 attempts
	deadline := time.After(2 * time.Second)
	for fc.getAttempts() < 2 {
		select {
		case <-deadline:
			t.Fatalf("timed out; only %d attempts", fc.getAttempts())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	status := r.Status()
	if status.State != StateReconnecting {
		t.Errorf("state = %v, want %v", status.State, StateReconnecting)
	}
	if status.Attempts < 2 {
		t.Errorf("attempts = %d, want >= 2", status.Attempts)
	}
}

func TestReconnector_stopsOnNonRetryableError(t *testing.T) {
	fc := &fakeConnector{err: &ConnectionError{Kind: ErrAuth, Message: "auth failed"}}

	r, _ := newFastReconnector(fc.connect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	// Should stop after 1 attempt
	select {
	case <-done:
		// good, Run exited
	case <-time.After(2 * time.Second):
		t.Fatal("timed out; Reconnector should have stopped on auth failure")
	}

	status := r.Status()
	if status.State != StateFailed {
		t.Errorf("state = %v, want %v", status.State, StateFailed)
	}
	if status.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", status.Attempts)
	}
	if status.LastError == "" {
		t.Error("expected LastError to be set")
	}
}

func TestReconnector_resetsBackoffOnSuccess(t *testing.T) {
	// Start with network error, then succeed
	fc := &fakeConnector{err: &ConnectionError{Kind: ErrNetwork, Message: "refused"}}

	r, _ := newFastReconnector(fc.connect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx)

	// Wait for a few attempts
	deadline := time.After(2 * time.Second)
	for fc.getAttempts() < 2 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for attempts")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Now make it succeed
	fc.setError(nil)

	// Wait for connected state
	deadline = time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for connected state")
		default:
			if r.Status().State == StateConnected {
				goto done
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
done:

	status := r.Status()
	if status.State != StateConnected {
		t.Errorf("state = %v, want %v", status.State, StateConnected)
	}
	if status.Attempts != 0 {
		t.Errorf("attempts = %d, want 0 (reset on success)", status.Attempts)
	}
}

func TestReconnector_contextCancellationStopsReconnection(t *testing.T) {
	fc := &fakeConnector{err: &ConnectionError{Kind: ErrNetwork, Message: "refused"}}

	r, _ := newFastReconnector(fc.connect)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	// Let it start reconnecting
	deadline := time.After(2 * time.Second)
	for fc.getAttempts() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestReconnector_attemptCountTracked(t *testing.T) {
	fc := &fakeConnector{err: &ConnectionError{Kind: ErrNetwork, Message: "refused"}}

	r, _ := newFastReconnector(fc.connect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx)

	// Wait for 3 attempts
	deadline := time.After(2 * time.Second)
	for fc.getAttempts() < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out; only %d attempts", fc.getAttempts())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	status := r.Status()
	if status.Attempts < 3 {
		t.Errorf("status.Attempts = %d, want >= 3", status.Attempts)
	}
}

func TestReconnector_backoffDelaysAreCorrect(t *testing.T) {
	// Fail 6 times to verify the full backoff schedule is used
	callCount := 0
	fc := &fakeConnector{}
	fc.err = &ConnectionError{Kind: ErrNetwork, Message: "refused"}

	r, dr := newFastReconnector(func(ctx context.Context) error {
		e := fc.connect(ctx)
		fc.mu.Lock()
		callCount = fc.attempts
		fc.mu.Unlock()
		// Succeed on the 7th attempt
		if callCount >= 7 {
			return nil
		}
		return e
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	delays := dr.getDelays()
	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
	}
	if len(delays) != len(expected) {
		t.Fatalf("got %d delays, want %d: %v", len(delays), len(expected), delays)
	}
	for i, want := range expected {
		if delays[i] != want {
			t.Errorf("delay[%d] = %v, want %v", i, delays[i], want)
		}
	}
}

func TestReconnector_hostKeyMismatchStopsReconnection(t *testing.T) {
	fc := &fakeConnector{err: &ConnectionError{Kind: ErrHostKey, Message: "host key mismatch"}}

	r, _ := newFastReconnector(fc.connect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out; should stop on host key mismatch")
	}

	if fc.getAttempts() != 1 {
		t.Errorf("attempts = %d, want 1", fc.getAttempts())
	}
	status := r.Status()
	if status.State != StateFailed {
		t.Errorf("state = %v, want %v", status.State, StateFailed)
	}
}

// --- Keepalive tests ---

// fakePinger tracks ping calls.
type fakePinger struct {
	mu       sync.Mutex
	calls    int
	err      error
}

func (f *fakePinger) ping(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.err
}

func (f *fakePinger) getCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakePinger) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func TestKeepalive_detectsThreeConsecutiveFailures(t *testing.T) {
	fp := &fakePinger{err: fmt.Errorf("ping failed")}
	triggered := make(chan struct{}, 1)

	ka := NewKeepalive(fp.ping, func() {
		select {
		case triggered <- struct{}{}:
		default:
		}
	})
	// Use fast interval for testing
	ka.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ka.Run(ctx)

	select {
	case <-triggered:
		// good, reconnect was triggered
	case <-time.After(2 * time.Second):
		t.Fatal("timed out; expected reconnect trigger after 3 keepalive failures")
	}

	if fp.getCalls() < 3 {
		t.Errorf("expected at least 3 ping calls, got %d", fp.getCalls())
	}
}

func TestKeepalive_resetsCounterOnSuccess(t *testing.T) {
	// Pinger that fails twice, succeeds once, then fails twice again.
	// Should never reach 3 consecutive failures.
	callNum := 0
	var mu sync.Mutex

	ping := func(ctx context.Context) error {
		mu.Lock()
		callNum++
		n := callNum
		mu.Unlock()

		// Pattern: fail, fail, success, fail, fail, success, ...
		// (every 3rd call succeeds)
		if n%3 == 0 {
			return nil
		}
		return fmt.Errorf("ping failed")
	}

	triggered := make(chan struct{}, 10)
	ka := NewKeepalive(ping, func() {
		triggered <- struct{}{}
	})
	ka.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ka.Run(ctx)

	// Let it run for enough calls (at least 9 = 3 full cycles of fail-fail-success)
	time.Sleep(120 * time.Millisecond)
	cancel()

	// Should never have triggered because consecutive failures never reached 3
	select {
	case <-triggered:
		t.Error("should not have triggered reconnect; success should reset failure counter")
	default:
		// good
	}
}

func TestKeepalive_contextCancellationStops(t *testing.T) {
	fp := &fakePinger{}

	ka := NewKeepalive(fp.ping, func() {})
	ka.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ka.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive Run did not return after context cancellation")
	}
}
