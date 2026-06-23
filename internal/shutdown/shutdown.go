// Package shutdown provides ordered graceful shutdown coordination.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// StepFunc is a shutdown step that receives a context with the hard timeout
// deadline. Implementations should respect context cancellation.
type StepFunc func(ctx context.Context) error

type step struct {
	name string
	fn   StepFunc
}

// Sequence holds an ordered list of shutdown steps and a hard timeout.
// Steps execute sequentially in the order they were added. If a step
// returns an error, execution continues with the next step (all errors
// are collected). If the total time exceeds the hard timeout, remaining
// steps are skipped and Run returns an error.
type Sequence struct {
	timeout time.Duration
	steps   []step
}

// NewSequence creates a Sequence with the given hard timeout for the
// entire shutdown process.
func NewSequence(timeout time.Duration) *Sequence {
	return &Sequence{timeout: timeout}
}

// Add appends a named shutdown step. Steps run in the order they are added.
func (s *Sequence) Add(name string, fn StepFunc) {
	s.steps = append(s.steps, step{name: name, fn: fn})
}

// Run executes all shutdown steps sequentially within the hard timeout.
// It returns a joined error of all step failures, or a timeout error if
// the hard deadline is exceeded.
func (s *Sequence) Run(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, s.timeout)
	defer cancel()

	type result struct {
		errs []error
	}

	done := make(chan result, 1)

	go func() {
		var errs []error
		for _, st := range s.steps {
			if ctx.Err() != nil {
				errs = append(errs, fmt.Errorf("shutdown timed out before step %q", st.name))
				break
			}
			slog.Info("shutdown: starting", "step", st.name)
			if err := st.fn(ctx); err != nil {
				slog.Error("shutdown: step failed", "step", st.name, "error", err)
				errs = append(errs, fmt.Errorf("step %q: %w", st.name, err))
			} else {
				slog.Info("shutdown: completed", "step", st.name)
			}
		}
		done <- result{errs: errs}
	}()

	select {
	case r := <-done:
		return errors.Join(r.errs...)
	case <-ctx.Done():
		return fmt.Errorf("graceful shutdown timed out after %v", s.timeout)
	}
}
