package clock

import (
	"context"
	"testing"
	"time"
)

var t0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func TestFake_Now(t *testing.T) {
	clk := NewFake(t0)
	if got := clk.Now(); !got.Equal(t0) {
		t.Errorf("Now() = %v, want %v", got, t0)
	}
}

func TestFake_Advance(t *testing.T) {
	clk := NewFake(t0)
	clk.Advance(5 * time.Second)
	want := t0.Add(5 * time.Second)
	if got := clk.Now(); !got.Equal(want) {
		t.Errorf("Now() after Advance = %v, want %v", got, want)
	}
}

func TestFake_Sleep_NonBlocking(t *testing.T) {
	clk := NewFake(t0)
	err := clk.Sleep(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("Sleep: %v", err)
	}
	want := t0.Add(10 * time.Second)
	if got := clk.Now(); !got.Equal(want) {
		t.Errorf("Now() after Sleep = %v, want %v", got, want)
	}
}

func TestFake_Sleep_RecordsDurations(t *testing.T) {
	clk := NewFake(t0)
	clk.Sleep(context.Background(), 1*time.Second)
	clk.Sleep(context.Background(), 2*time.Second)
	clk.Sleep(context.Background(), 4*time.Second)

	sleeps := clk.Sleeps()
	if len(sleeps) != 3 {
		t.Fatalf("len(Sleeps) = %d, want 3", len(sleeps))
	}
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for i, d := range want {
		if sleeps[i] != d {
			t.Errorf("Sleeps[%d] = %v, want %v", i, sleeps[i], d)
		}
	}
}

func TestFake_Sleep_RespectsContext(t *testing.T) {
	clk := NewFake(t0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := clk.Sleep(ctx, 10*time.Second)
	if err == nil {
		t.Fatal("Sleep should return error on cancelled context")
	}
}

func TestFake_Ticker_FiresOnAdvance(t *testing.T) {
	clk := NewFake(t0)
	ticker := clk.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Not yet at the first tick
	select {
	case <-ticker.C():
		t.Fatal("ticker should not fire before first interval")
	default:
	}

	// Advance to exactly the first tick
	clk.Advance(5 * time.Second)
	select {
	case <-ticker.C():
		// good
	default:
		t.Fatal("ticker should fire at first interval")
	}
}

func TestFake_Ticker_MultipleFiresOnLargeAdvance(t *testing.T) {
	clk := NewFake(t0)
	ticker := clk.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Advance by 7s — should cross tick at 3s and 6s.
	// Channel is buffered at 1, so only 1 tick is buffered (rest dropped).
	clk.Advance(7 * time.Second)
	select {
	case <-ticker.C():
	default:
		t.Fatal("expected at least one tick")
	}
}

func TestFake_Ticker_StopPreventsFireing(t *testing.T) {
	clk := NewFake(t0)
	ticker := clk.NewTicker(1 * time.Second)
	ticker.Stop()

	clk.Advance(5 * time.Second)
	select {
	case <-ticker.C():
		t.Fatal("stopped ticker should not fire")
	default:
	}
}

func TestFake_SleepFiresTickers(t *testing.T) {
	clk := NewFake(t0)
	ticker := clk.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Sleep advances time, which should fire tickers too
	clk.Sleep(context.Background(), 3*time.Second)
	select {
	case <-ticker.C():
	default:
		t.Fatal("Sleep should fire tickers when crossing tick boundary")
	}
}

func TestReal_Satisfies_Interface(t *testing.T) {
	var _ Clock = Real{}
}

func TestFake_Satisfies_Interface(t *testing.T) {
	var _ Clock = &Fake{}
}
