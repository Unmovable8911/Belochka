package clock

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Fake is a controllable Clock for testing.
//
// Sleep is non-blocking: it advances the internal time by the requested
// duration and returns immediately, making loops that sleep between
// iterations (like Reconnector) run at full speed in tests.
//
// Tickers fire when Advance (or Sleep) moves the clock past a tick
// boundary. Call Advance from the test goroutine to drive loops that
// block on ticker.C().
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	sleeps  []time.Duration
	tickers []*FakeTicker
}

// NewFake creates a Fake clock starting at the given time.
func NewFake(start time.Time) *Fake {
	return &Fake{now: start}
}

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) NewTicker(d time.Duration) Ticker {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &FakeTicker{
		interval: d,
		nextTick: f.now.Add(d),
		c:        make(chan time.Time, 1),
	}
	f.tickers = append(f.tickers, t)
	return t
}

// Sleep advances the clock by d and returns immediately.
// It records the duration for later inspection via Sleeps().
func (f *Fake) Sleep(ctx context.Context, d time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.mu.Lock()
	f.sleeps = append(f.sleeps, d)
	f.now = f.now.Add(d)
	f.fireTickers()
	f.mu.Unlock()
	return nil
}

// Advance moves the clock forward by d and fires any tickers whose
// next tick time has been reached. Use this from the test goroutine
// to drive loops that block on ticker.C().
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.fireTickers()
	f.mu.Unlock()
}

// Sleeps returns the durations passed to Sleep, in order.
func (f *Fake) Sleeps() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]time.Duration, len(f.sleeps))
	copy(out, f.sleeps)
	return out
}

// fireTickers sends on each non-stopped ticker's channel for every
// tick boundary the clock has crossed. Must be called with f.mu held.
func (f *Fake) fireTickers() {
	for _, t := range f.tickers {
		if t.stopped.Load() {
			continue
		}
		for !f.now.Before(t.nextTick) {
			select {
			case t.c <- f.now:
			default:
			}
			t.nextTick = t.nextTick.Add(t.interval)
		}
	}
}

// FakeTicker is a controllable Ticker for testing.
type FakeTicker struct {
	interval time.Duration
	nextTick time.Time
	c        chan time.Time
	stopped  atomic.Bool
}

func (t *FakeTicker) C() <-chan time.Time { return t.c }

func (t *FakeTicker) Stop() { t.stopped.Store(true) }
