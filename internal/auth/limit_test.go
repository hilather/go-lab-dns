package auth

import (
	"testing"
	"time"
)

func TestLimiterBurstThenRefill(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(1, 2, func() time.Time { return now })
	if !l.Allow("a") {
		t.Fatal("burst first")
	}
	if !l.Allow("a") {
		t.Fatal("burst second")
	}
	if l.Allow("a") {
		t.Fatal("over burst")
	}
	now = now.Add(time.Second)
	if !l.Allow("a") {
		t.Fatal("refill")
	}
	if !l.Allow("b") {
		t.Fatal("other key")
	}
}

func TestLimiterDisabled(t *testing.T) {
	l := NewLimiter(-1, 0, nil)
	for i := 0; i < 100; i++ {
		if !l.Allow("x") {
			t.Fatal("disabled")
		}
	}
}

func TestManagementLimiterDefaults(t *testing.T) {
	l := ManagementLimiter(0, 0, nil)
	if l.rate != DefaultMgmtRatePerSec || l.burst != DefaultMgmtBurst {
		t.Fatalf("%v %v", l.rate, l.burst)
	}
}
