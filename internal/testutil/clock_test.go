package testutil

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestFakeClockAdvanceAndTimer(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clk := NewFakeClock(start)
	if !clk.Now().Equal(start) {
		t.Fatalf("Now = %v, want %v", clk.Now(), start)
	}
	if clk.Monotonic() != 0 {
		t.Fatalf("Monotonic = %v, want 0", clk.Monotonic())
	}

	timer := clk.NewTimer(10 * time.Millisecond)
	select {
	case <-timer.C():
		t.Fatal("timer fired before Advance")
	default:
	}

	clk.Advance(10 * time.Millisecond)
	select {
	case got := <-timer.C():
		if !got.Equal(start.Add(10 * time.Millisecond)) {
			t.Fatalf("timer time = %v, want %v", got, start.Add(10*time.Millisecond))
		}
	default:
		t.Fatal("timer did not fire after Advance")
	}
	if clk.Monotonic() != 10*time.Millisecond {
		t.Fatalf("Monotonic = %v, want 10ms", clk.Monotonic())
	}
}

func TestFakeClockStopPreventsFire(t *testing.T) {
	clk := NewFakeClock(time.Unix(0, 0).UTC())
	timer := clk.NewTimer(time.Second)
	if !timer.Stop() {
		t.Fatal("Stop = false, want true")
	}
	if timer.Stop() {
		t.Fatal("second Stop = true, want false")
	}
	clk.Advance(time.Second)
	select {
	case <-timer.C():
		t.Fatal("stopped timer fired")
	default:
	}
}

func TestFakeClockConcurrentAdvanceAndNow(t *testing.T) {
	clk := NewFakeClock(time.Unix(0, 0).UTC())
	rng := NewSeededRand(1)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				clk.Advance(time.Microsecond)
				_ = clk.Now()
				_ = clk.Monotonic()
				_ = rng.Uint64()
			}
		}()
	}
	wg.Wait()
	if clk.Monotonic() != 8000*time.Microsecond {
		t.Fatalf("Monotonic = %v, want 8ms", clk.Monotonic())
	}
}

func TestCleanupContextCancels(t *testing.T) {
	var ctx context.Context
	t.Run("live", func(t *testing.T) {
		ctx = Context(t)
		if ctx.Err() != nil {
			t.Fatal("context canceled during the owning test")
		}
	})
	if ctx.Err() == nil {
		t.Fatal("context was not canceled after the owning test ended")
	}
}
