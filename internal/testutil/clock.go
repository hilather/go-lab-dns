package testutil

import (
	"sync"
	"time"
)

// Clock is an injectable time source.
type Clock interface {
	Now() time.Time
	Monotonic() time.Duration
	NewTimer(d time.Duration) Timer
}

// Timer is a context-free timer that fake clocks can fire by Advance.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// SystemClock uses the process wall clock and a process-start monotonic base.
type SystemClock struct{}

var systemStart = time.Now()

// Now returns the current wall time.
func (SystemClock) Now() time.Time { return time.Now() }

// Monotonic returns time since process start of this package.
func (SystemClock) Monotonic() time.Duration { return time.Since(systemStart) }

// NewTimer returns a timer backed by time.NewTimer.
func (SystemClock) NewTimer(d time.Duration) Timer {
	return stdTimer{t: time.NewTimer(d)}
}

type stdTimer struct {
	t *time.Timer
}

func (s stdTimer) C() <-chan time.Time { return s.t.C }

func (s stdTimer) Stop() bool { return s.t.Stop() }

// FakeClock is a mutex-protected clock for deterministic tests.
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	mono   time.Duration
	timers []*fakeTimer
}

// NewFakeClock returns a clock starting at now with monotonic zero.
func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{now: now}
}

// Now returns the fake wall time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Monotonic returns the fake monotonic duration.
func (c *FakeClock) Monotonic() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mono
}

// NewTimer schedules a timer that fires when Advance reaches d.
func (c *FakeClock) NewTimer(d time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{
		clock:  c,
		fireAt: c.mono + d,
		ch:     make(chan time.Time, 1),
	}
	if d <= 0 {
		t.stopped = true
		t.ch <- c.now
		return t
	}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock forward and fires due timers.
func (c *FakeClock) Advance(d time.Duration) {
	if d < 0 {
		panic("testutil: FakeClock.Advance: negative duration")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.mono += d
	kept := c.timers[:0]
	for _, t := range c.timers {
		if t.stopped {
			continue
		}
		if t.fireAt <= c.mono {
			t.stopped = true
			select {
			case t.ch <- c.now:
			default:
			}
			continue
		}
		kept = append(kept, t)
	}
	c.timers = kept
}

type fakeTimer struct {
	clock   *FakeClock
	fireAt  time.Duration
	ch      chan time.Time
	stopped bool
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}
