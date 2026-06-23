package clock

import (
	"context"
	"time"
)

// Ticker abstracts time.Ticker for testability.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Clock abstracts time operations for testability.
type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
	Sleep(ctx context.Context, d time.Duration) error
}

// Real implements Clock using the standard library.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

func (Real) NewTicker(d time.Duration) Ticker {
	return &realTicker{time.NewTicker(d)}
}

func (Real) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type realTicker struct{ t *time.Ticker }

func (rt *realTicker) C() <-chan time.Time { return rt.t.C }
func (rt *realTicker) Stop()              { rt.t.Stop() }
