package ssh

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"belochka/internal/clock"
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

var t0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// newFastReconnector creates a Reconnector with a FakeClock for testing.
// Sleep is non-blocking, so the reconnection loop runs at full speed.
func newFastReconnector(connect ConnectFunc) (*Reconnector, *clock.Fake) {
	clk := clock.NewFake(t0)
	r := NewReconnector(connect)
	r.clock = clk
	return r, clk
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
	callCount := 0
	fc := &fakeConnector{}
	fc.err = &ConnectionError{Kind: ErrNetwork, Message: "refused"}

	r, clk := newFastReconnector(func(ctx context.Context) error {
		e := fc.connect(ctx)
		fc.mu.Lock()
		callCount = fc.attempts
		fc.mu.Unlock()
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

	sleeps := clk.Sleeps()
	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
	}
	if len(sleeps) != len(expected) {
		t.Fatalf("got %d sleeps, want %d: %v", len(sleeps), len(expected), sleeps)
	}
	for i, want := range expected {
		if sleeps[i] != want {
			t.Errorf("sleep[%d] = %v, want %v", i, sleeps[i], want)
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

	clk := clock.NewFake(t0)
	ka := NewKeepalive(fp.ping, func() {
		select {
		case triggered <- struct{}{}:
		default:
		}
	})
	ka.clock = clk
	ka.interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ka.Run(ctx)
	time.Sleep(time.Millisecond) // let goroutine register ticker

	for i := 0; i < KeepaliveFailureThreshold; i++ {
		clk.Advance(10 * time.Second)
		time.Sleep(time.Millisecond) // let goroutine process tick
	}

	select {
	case <-triggered:
	default:
		t.Fatal("expected reconnect trigger after 3 keepalive failures")
	}

	if fp.getCalls() < 3 {
		t.Errorf("expected at least 3 ping calls, got %d", fp.getCalls())
	}
}

func TestKeepalive_resetsCounterOnSuccess(t *testing.T) {
	callNum := 0
	var mu sync.Mutex

	ping := func(ctx context.Context) error {
		mu.Lock()
		callNum++
		n := callNum
		mu.Unlock()

		if n%3 == 0 {
			return nil
		}
		return fmt.Errorf("ping failed")
	}

	triggered := make(chan struct{}, 10)
	clk := clock.NewFake(t0)
	ka := NewKeepalive(ping, func() {
		triggered <- struct{}{}
	})
	ka.clock = clk
	ka.interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ka.Run(ctx)
	time.Sleep(time.Millisecond)

	for i := 0; i < 9; i++ {
		clk.Advance(10 * time.Second)
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case <-triggered:
		t.Error("should not have triggered reconnect; success should reset failure counter")
	default:
	}
}

func TestKeepalive_contextCancellationStops(t *testing.T) {
	fp := &fakePinger{}

	clk := clock.NewFake(t0)
	ka := NewKeepalive(fp.ping, func() {})
	ka.clock = clk
	ka.interval = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ka.Run(ctx)
		close(done)
	}()

	// Let the goroutine start and block on ticker
	time.Sleep(time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive Run did not return after context cancellation")
	}
}
