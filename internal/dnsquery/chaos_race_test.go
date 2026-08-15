package dnsquery

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestChaosRaceDecideAndSwap(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{
		delayPolicy("slow", "a1", model.PhaseBeforeResponse, time.Millisecond, 0),
		rcodePolicy("rc", "SERVFAIL", nil),
	}
	// Two policies compose: delay + rcode. rcodePolicy also attaches to a1.
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	eng := chaos.NewEngine(nil, nil)
	h := NewOpts(Opts{Store: store, Engine: eng})

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
			_, _, _ = h.ServeDNS(context.Background(), &q)
		}()
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			next := compileSnap(t, st)
			store.Swap(next)
		}()
	}
	wg.Wait()
	if eng.Budgets().InFlight() != 0 {
		t.Fatalf("leaked %d", eng.Budgets().InFlight())
	}
}

func TestChaosSoakShort(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{delayPolicy("slow", "a1", model.PhaseBeforeResolution, time.Millisecond, 0)}
	h := handlerFromState(t, st, nil)
	q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
	var wg sync.WaitGroup
	errCh := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			qq := q
			_, _, err := h.ServeDNS(context.Background(), &qq)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}
