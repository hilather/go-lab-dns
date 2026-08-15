package chaos

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// EmergencyDisable sets the store inhibit bit, stamps the active snapshot,
// and cancels outstanding delay reservations. YAML cannot relax the bit.
func EmergencyDisable(store *snapshot.Store, eng *Engine) *snapshot.Snapshot {
	if store == nil {
		return nil
	}
	store.SetEmergencyChaosOff(true)
	snap := store.StampEmergency()
	if eng != nil {
		eng.CancelDelays()
	}
	return snap
}

// ServeSignals treats SIGUSR1 as emergency disable. SIGUSR2 is reserved
// and ignored in first GA. The function returns when ctx is done.
func ServeSignals(ctx context.Context, sigs <-chan os.Signal, store *snapshot.Store, eng *Engine) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-sigs:
			if !ok {
				return
			}
			if s == syscall.SIGUSR1 {
				EmergencyDisable(store, eng)
			}
		}
	}
}

// NotifyUSR1 arms process signal notification for SIGUSR1 and SIGUSR2.
// The returned stop func unregisters the channel.
func NotifyUSR1() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1, syscall.SIGUSR2)
	return ch, func() { signal.Stop(ch) }
}

// EnvChaosDisable reports LABDNS_CHAOS_DISABLE=1/true (startup inhibit).
func EnvChaosDisable() bool {
	v := os.Getenv("LABDNS_CHAOS_DISABLE")
	return v == "1" || v == "true" || v == "TRUE" || v == "yes"
}
