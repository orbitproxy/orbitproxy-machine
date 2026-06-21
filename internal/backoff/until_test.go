package backoff

import (
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func TestFastBackoffManagerFirstCallReturnsDuration(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.May, 8, 12, 30, 0, 0, time.UTC)
	clk := &fakeClock{now: start}
	manager := newFastBackoffManagerWithClock(FastBackoffOptions{
		Duration: time.Second,
	}, clk).(*fastBackoffImpl)

	if got := manager.Backoff(0, false); got != time.Second {
		t.Fatalf("first Backoff = %v, want 1s", got)
	}
	if !manager.lastCalledTime.Equal(start) {
		t.Fatalf("lastCalledTime = %v, want %v", manager.lastCalledTime, start)
	}
}

func TestUntilStopsOnStopCh(t *testing.T) {
	t.Parallel()

	stopCh := make(chan struct{})
	calls := 0
	done := make(chan struct{})

	go func() {
		Until(func() { calls++ }, 10*time.Millisecond, stopCh)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	close(stopCh)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Until did not stop")
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 calls, got %d", calls)
	}
}
