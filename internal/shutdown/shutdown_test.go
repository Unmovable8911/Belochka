package shutdown_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"belochka/internal/shutdown"
)

// recorder tracks the order in which shutdown steps execute.
type recorder struct {
	mu    sync.Mutex
	order []string
}

func (r *recorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, name)
}

func (r *recorder) steps() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]string, len(r.order))
	copy(cp, r.order)
	return cp
}

func TestShutdownExecutesStepsInOrder(t *testing.T) {
	rec := &recorder{}

	seq := shutdown.NewSequence(10 * time.Second)
	seq.Add("http", func(ctx context.Context) error {
		rec.record("http")
		return nil
	})
	seq.Add("websocket", func(ctx context.Context) error {
		rec.record("websocket")
		return nil
	})
	seq.Add("collectors", func(ctx context.Context) error {
		rec.record("collectors")
		return nil
	})
	seq.Add("ssh", func(ctx context.Context) error {
		rec.record("ssh")
		return nil
	})
	seq.Add("sqlite", func(ctx context.Context) error {
		rec.record("sqlite")
		return nil
	})

	err := seq.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := rec.steps()
	want := []string{"http", "websocket", "collectors", "ssh", "sqlite"}
	if len(got) != len(want) {
		t.Fatalf("step count: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShutdownContinuesOnStepError(t *testing.T) {
	rec := &recorder{}

	seq := shutdown.NewSequence(10 * time.Second)
	seq.Add("http", func(ctx context.Context) error {
		rec.record("http")
		return errors.New("http close error")
	})
	seq.Add("sqlite", func(ctx context.Context) error {
		rec.record("sqlite")
		return nil
	})

	err := seq.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from Run")
	}

	// Both steps should have executed despite the first error.
	got := rec.steps()
	if len(got) != 2 {
		t.Fatalf("expected 2 steps executed, got %d: %v", len(got), got)
	}
}

func TestShutdownHardTimeoutForceStops(t *testing.T) {
	seq := shutdown.NewSequence(100 * time.Millisecond) // very short timeout
	seq.Add("stalled", func(ctx context.Context) error {
		// This step blocks forever until context is cancelled.
		<-ctx.Done()
		return ctx.Err()
	})

	start := time.Now()
	err := seq.Run(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hard timeout")
	}

	// Should complete within ~200ms (100ms timeout + margin).
	if elapsed > 500*time.Millisecond {
		t.Errorf("shutdown took %v, expected ~100ms hard timeout", elapsed)
	}
}
