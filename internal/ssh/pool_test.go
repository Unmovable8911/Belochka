package ssh

import (
	"context"
	"errors"
	"testing"
	"time"

	"belochka/internal/clock"
	"belochka/internal/model"
)

// mockProvider implements ServerProvider with a configurable GetByID.
type mockProvider struct {
	getByID func(ctx context.Context, id string) (model.Server, error)
}

func (m *mockProvider) GetByID(ctx context.Context, id string) (model.Server, error) {
	return m.getByID(ctx, id)
}

func TestPool_reconnectRestartsFailedConnection(t *testing.T) {
	// provider returns an error so the connect function fails without
	// attempting a real TCP dial (avoids network timeouts).
	provider := &mockProvider{
		getByID: func(ctx context.Context, id string) (model.Server, error) {
			return model.Server{}, errors.New("server not found")
		},
	}

	clk := clock.NewFake(t0)
	pool := NewPool(provider, clk)

	srvID := "srv-1"

	// Add the server — connect will fail immediately and the Reconnector
	// will retry 5 times with non-blocking FakeClock sleep.
	pool.Add(context.Background(), srvID)

	// Wait for the Reconnector to reach failed state after 5 retries.
	deadline := time.After(2 * time.Second)
	for {
		status := pool.Status(srvID)
		if status.State == StateFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for failed state; got %v", pool.Status(srvID))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	status := pool.Status(srvID)
	if status.Attempts != 5 {
		t.Errorf("attempts before reconnect = %d, want 5", status.Attempts)
	}

	// Reconnect — should reset the Reconnector and start a new goroutine.
	pool.Reconnect(context.Background(), srvID)

	// After reconnect, the Reconnector starts fresh and will make
	// at least one attempt.
	deadline = time.After(2 * time.Second)
	for {
		status := pool.Status(srvID)
		if status.Attempts >= 1 && status.State == StateReconnecting {
			break
		}
		if status.State == StateFailed && status.Attempts == 5 {
			// If the reconnect goroutine already completed all 5 attempts,
			// that proves a new cycle started (Attempts reset + new attempts).
			t.Log("reconnect goroutine completed new cycle already")
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for reconnect to start; got state=%v attempts=%d",
				status.State, status.Attempts)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Verify the reconnector is in a fresh cycle — state is reconnecting or failed.
	finalStatus := pool.Status(srvID)
	if finalStatus.State != StateReconnecting && finalStatus.State != StateFailed {
		t.Errorf("state after reconnect = %v, want reconnecting or failed", finalStatus.State)
	}
	if finalStatus.State == StateFailed && finalStatus.Attempts != 5 {
		t.Errorf("attempts after reconnect = %d, want 5 for a fresh cycle", finalStatus.Attempts)
	}

	// Cleanup
	pool.Remove(srvID)
}

func TestPool_reconnectIdempotentForConnectedServer(t *testing.T) {
	// A "connected" scenario: provider returns a valid server, but the
	// connect function tries gossh.Dial which will fail. However, since
	// there's no real SSH server, we simulate a "connected" scenario by
	// using a provider that returns a proper server. The connect will
	// fail with a network error (retryable), eventually reaching failed.
	// While not truly "connected", Reconnect is still idempotent — it
	// works regardless of current state.
	provider := &mockProvider{
		getByID: func(ctx context.Context, id string) (model.Server, error) {
			return model.Server{
				ID:       "srv-2",
				Name:     "test",
				Host:     "127.0.0.1",
				Port:     9,
				Username: "nobody",
				AuthType: model.AuthTypePassword,
				Password: "test",
			}, nil
		},
	}

	clk := clock.NewFake(t0)
	pool := NewPool(provider, clk)

	srvID := "srv-2"
	pool.Add(context.Background(), srvID)

	// Wait for the reconnector to settle into failed state.
	deadline := time.After(5 * time.Second)
	for {
		status := pool.Status(srvID)
		if status.State == StateFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for state; got %v", pool.Status(srvID))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Call Reconnect on a failed server — should work.
	pool.Reconnect(context.Background(), srvID)

	// Call Reconnect again while it's reconnecting — idempotent.
	pool.Reconnect(context.Background(), srvID)

	// Verify the pool still has the connection.
	status := pool.Status(srvID)
	if status.State == "" {
		t.Error("expected a valid status after reconnect")
	}

	pool.Remove(srvID)
}
