package chaos

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestEmergencyDisableStampsStore(t *testing.T) {
	store := snapshot.NewStore()
	store.Swap(&snapshot.Snapshot{Revision: "sha256:aa", Generation: 1})
	eng := NewEngine(nil, nil)
	called := false
	eng.Budgets().RegisterCancel(func() { called = true })
	snap := EmergencyDisable(store, eng)
	if snap == nil || !snap.EmergencyChaosOff {
		t.Fatalf("snap=%+v", snap)
	}
	if !store.EmergencyChaosOff() {
		t.Fatal("store bit")
	}
	if !called {
		t.Fatal("cancel not invoked")
	}
}

func TestServeSignalsUSR1(t *testing.T) {
	store := snapshot.NewStore()
	store.Swap(&snapshot.Snapshot{Generation: 1})
	ch := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		ServeSignals(ctx, ch, store, nil)
		close(done)
	}()
	ch <- syscall.SIGUSR2 // ignored
	ch <- syscall.SIGUSR1
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if live := store.Load(); live != nil && live.EmergencyChaosOff {
			cancel()
			<-done
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	t.Fatal("SIGUSR1 did not set emergency bit")
}

func TestEnvChaosDisable(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "Yes", " yes "} {
		t.Setenv("LABDNS_CHAOS_DISABLE", v)
		if !EnvChaosDisable() {
			t.Fatalf("expected true for %q", v)
		}
	}
	for _, v := range []string{"0", "false", "", "no"} {
		t.Setenv("LABDNS_CHAOS_DISABLE", v)
		if EnvChaosDisable() {
			t.Fatalf("expected false for %q", v)
		}
	}
}
