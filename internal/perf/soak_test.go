package perf

import (
	"context"
	"flag"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// soak is CI-safe by default. Operators pass -soak=30m or
// LABDNS_SOAK_DURATION=30m for the pre-GA long run.
var soak = flag.Duration("soak", DefaultSoak, "soak duration (CI default 2s; use 30m before GA)")

func soakDuration(t *testing.T) time.Duration {
	t.Helper()
	d := *soak
	if env := os.Getenv("LABDNS_SOAK_DURATION"); env != "" {
		parsed, err := time.ParseDuration(env)
		if err != nil {
			t.Fatalf("LABDNS_SOAK_DURATION: %v", err)
		}
		d = parsed
	}
	if testing.Short() && d > 500*time.Millisecond {
		d = 500 * time.Millisecond
	}
	if d < 50*time.Millisecond {
		d = 50 * time.Millisecond
	}
	return d
}

func TestSoakSwapsAndExpiry(t *testing.T) {
	d := soakDuration(t)
	t.Log(CaptureEnv().String())
	t.Logf("soak duration %s", d)

	const soakDelay = 80 * time.Millisecond
	st := LabState("")
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = soakDelay
	// Start with the delay policy already expired so the first expiry
	// flip is a no-op until we extend it.
	expired := time.Now().UTC().Add(-time.Second)
	st.Spec.Chaos.Policies[1].ExpiresAt = &expired
	lab := NewLab(t, Options{State: st, StartServer: true})
	svc := app.New(app.Options{Store: lab.Store, Cache: lab.Cache, Engine: lab.Engine, Clock: lab.Clock})
	actor := app.Actor{ID: "soak", Class: "token", Scopes: []string{
		"dns.write", "dns.admin", "dns.chaos.write", "dns.chaos.activate", "dns.chaos.emergency",
	}}

	var (
		queries      atomic.Int64
		delayQueries atomic.Int64
		swaps        atomic.Int64
		expiries     atomic.Int64
		badAnswer    atomic.Int64
		partial      atomic.Int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := uint16(1)
			for ctx.Err() == nil {
				id++
				raw, err := EncodeQuery(id, QueryExact(), nil)
				if err != nil {
					badAnswer.Add(1)
					continue
				}
				out, err := DialUDP(lab.UDPAddr(), raw, 300*time.Millisecond)
				if err != nil || out == nil {
					continue
				}
				msg, err := unpackQuiet(out)
				if err != nil {
					partial.Add(1)
					continue
				}
				queries.Add(1)
				if msg.RCode != model.RCodeNoError {
					badAnswer.Add(1)
					continue
				}
				if len(msg.Answers) != 1 || msg.Answers[0].Type != model.TypeA {
					partial.Add(1)
					continue
				}
				ip := msg.Answers[0].Data
				if ip != "10.42.0.80" && ip != "10.42.0.81" {
					partial.Add(1)
				}
			}
		}()
	}

	// Hit the delay policy (ns-a / slow-ns) so activate/expiry is not
	// mutation-API smoke: answers must stay complete while the policy flips.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := uint16(200)
			for ctx.Err() == nil {
				id++
				raw, err := EncodeQuery(id, QueryDelay(), nil)
				if err != nil {
					badAnswer.Add(1)
					continue
				}
				out, err := DialUDP(lab.UDPAddr(), raw, 400*time.Millisecond)
				if err != nil || out == nil {
					continue
				}
				msg, err := unpackQuiet(out)
				if err != nil {
					partial.Add(1)
					continue
				}
				delayQueries.Add(1)
				if msg.RCode != model.RCodeNoError {
					badAnswer.Add(1)
					continue
				}
				if len(msg.Answers) != 1 || msg.Answers[0].Type != model.TypeA || msg.Answers[0].Data != "10.42.0.53" {
					partial.Add(1)
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		flip := false
		for ctx.Err() == nil {
			next := LabState("")
			next.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = soakDelay
			ip := "10.42.0.80"
			if flip {
				ip = "10.42.0.81"
			}
			flip = !flip
			for i := range next.Spec.Zones[0].Records {
				if next.Spec.Zones[0].Records[i].ID == "www-a" {
					next.Spec.Zones[0].Records[i].Values = []string{ip}
				}
			}
			if _, err := compileSwap(lab, next); err != nil {
				t.Errorf("swap: %v", err)
			} else {
				swaps.Add(1)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		on := false
		var n uint64
		for ctx.Err() == nil {
			snap := lab.Store.Load()
			if snap == nil {
				continue
			}
			n++
			exp := time.Now().UTC().Add(80 * time.Millisecond)
			key := "soak-" + string(snap.Revision) + "-" + strconv.FormatUint(n, 10)
			if on {
				_, err := svc.SetChaosExpiry(context.Background(), actor, app.ExpiryIn{
					PolicyID:         "slow-ns",
					ExpectedRevision: snap.Revision,
					IdempotencyKey:   key,
					Reason:           "soak expiry",
					ExpiresAt:        &exp,
				})
				if err == nil {
					expiries.Add(1)
				}
			} else {
				_, err := svc.ActivateChaos(context.Background(), actor, app.ActivationIn{
					PolicyID:         "slow-ns",
					ExpectedRevision: snap.Revision,
					IdempotencyKey:   key,
					Reason:           "soak activate",
					ExpiresAt:        &exp,
				})
				if err == nil {
					expiries.Add(1)
				}
			}
			on = !on
			select {
			case <-ctx.Done():
				return
			case <-time.After(40 * time.Millisecond):
			}
		}
	}()

	wg.Wait()

	if partial.Load() > 0 {
		t.Fatalf("partial answers during swap: %d", partial.Load())
	}
	if badAnswer.Load() > 0 {
		t.Fatalf("unexpected rcode during soak: %d", badAnswer.Load())
	}
	if swaps.Load() < 1 {
		t.Fatal("no snapshot swaps ran")
	}
	if queries.Load() < 1 {
		t.Fatal("no successful queries during soak")
	}
	if delayQueries.Load() < 1 {
		t.Fatal("no delay-name queries during soak")
	}
	if lab.Engine.Budgets().InFlight() != 0 {
		t.Fatalf("delay reservations leaked: %d", lab.Engine.Budgets().InFlight())
	}
	if lab.Cache != nil {
		if n := lab.Cache.Stats().Entries; n > SafeCacheMaxEntries {
			t.Fatalf("cache grew past cap: %d", n)
		}
	}

	// Expired delay must not hold ns.lab.example. for the configured duration.
	snap := lab.Store.Load()
	if snap == nil {
		t.Fatal("no snapshot after soak")
	}
	past := time.Now().UTC().Add(-time.Second)
	if _, err := svc.SetChaosExpiry(context.Background(), actor, app.ExpiryIn{
		PolicyID:         "slow-ns",
		ExpectedRevision: snap.Revision,
		IdempotencyKey:   "soak-final-expire",
		Reason:           "assert expiry takes effect",
		ExpiresAt:        &past,
	}); err != nil {
		t.Fatalf("final expire: %v", err)
	}
	t0 := time.Now()
	delayed := lab.Serve(t, QueryDelay())
	elapsed := time.Since(t0)
	if delayed.RCode != model.RCodeNoError || len(delayed.Answers) != 1 {
		t.Fatalf("expired delay answer %+v", delayed)
	}
	if elapsed > 30*time.Millisecond {
		t.Fatalf("expired delay still held the query for %s", elapsed)
	}

	t.Logf("queries=%d delay_queries=%d swaps=%d expiry_ops=%d", queries.Load(), delayQueries.Load(), swaps.Load(), expiries.Load())
}

func TestSoakNoGoroutineGrowth(t *testing.T) {
	d := soakDuration(t)
	if d > 3*time.Second && !testing.Short() {
		// The long flag already runs TestSoakSwapsAndExpiry; keep this
		// leak check CI-short so -soak=30m does not double the wall time.
		d = 2 * time.Second
	}
	lab := NewLab(t, Options{StartServer: true})
	runtime.GC()
	before := runtime.NumGoroutine()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		q := QueryExact()
		raw := PackQuery(t, 9, q, nil)
		_ = ExchangeUDP(t, lab.UDPAddr(), raw, 200*time.Millisecond)
		lab.Swap(t, LabState(""))
	}
	runtime.GC()
	time.Sleep(30 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+24 {
		t.Fatalf("goroutine growth before=%d after=%d", before, after)
	}
}

func compileSwap(lab *Lab, st *model.State) (*snapshot.Snapshot, error) {
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: lab.Clock})
	if err != nil {
		return nil, err
	}
	lab.Store.Swap(snap)
	return snap, nil
}

func unpackQuiet(raw []byte) (*dnswire.UpstreamMsg, error) {
	return dnswire.UnpackUpstream(raw)
}
