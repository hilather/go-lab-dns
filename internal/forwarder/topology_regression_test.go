package forwarder

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func churnSnapshot(generation model.Generation, id string, endpoint string) *snapshot.Snapshot {
	poolID := model.PoolID("pool-" + id)
	return &snapshot.Snapshot{Generation: generation, Forwarding: snapshot.ForwardingIndex{
		ByID:  map[model.PolicyID]*snapshot.CompiledPolicy{"pol": {ID: "pol", PoolID: poolID}},
		Pools: map[model.PoolID]*snapshot.CompiledPool{poolID: {ID: poolID, Strategy: model.StrategyRoundRobin, Upstreams: []snapshot.CompiledUpstream{{ID: model.UpstreamID(id), Endpoint: endpoint, Transport: model.TransportUDP}}}},
	}}
}

func TestForwarderRuntimeStateIsBoundedDuringTopologyChurn(t *testing.T) {
	rt := NewRuntime(nil, nil, nil, func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("unreachable") })
	for i := 1; i <= 100; i++ {
		snap := churnSnapshot(model.Generation(i), fmt.Sprintf("u%d", i), "127.0.0.1:1")
		if _, err := rt.Exchange(t.Context(), snap, query("x.example."), "pol"); err != nil {
			t.Fatal(err)
		}
		if len(rt.Health.byID) > 1 || len(rt.pick.rr) > 1 {
			t.Fatalf("generation %d retained historical state: health=%d counters=%d", i, len(rt.Health.byID), len(rt.pick.rr))
		}
	}
}

func TestOldForwarderCompletionCannotRestoreRemovedTopology(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			up := startFake(t)
			started, release := make(chan struct{}), make(chan struct{})
			done := make(chan model.Result, 1)
			rt := NewRuntime(nil, nil, nil, func(ctx context.Context, network, endpoint string) (net.Conn, error) {
				if endpoint == "old:53" {
					close(started)
					select {
					case <-release:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					if success {
						return defaultDial(ctx, network, up.UDPAddr())
					}
				}
				return nil, errors.New("unreachable")
			})
			old := churnSnapshot(1, "old", "old:53")
			go func() { result, _ := rt.Exchange(t.Context(), old, query("x.example."), "pol"); done <- result }()
			<-started
			current := churnSnapshot(2, "new", "new:53")
			if _, err := rt.Exchange(t.Context(), current, query("x.example."), "pol"); err != nil {
				t.Fatal(err)
			}
			close(release)
			if result := <-done; result.UpstreamID != "old" {
				t.Fatalf("old request stopped using its own snapshot: %+v", result)
			}
			if _, exists := rt.Health.byID["old"]; exists {
				t.Fatal("old completion restored removed upstream health")
			}
			if _, exists := rt.pick.rr["pool-old"]; exists {
				t.Fatal("removed round-robin pool retained")
			}
			// A request starting on an old snapshot still uses its pool without
			// restoring its counter or health history.
			old = churnSnapshot(1, "old", "late-old:53")
			if _, err := rt.Exchange(t.Context(), old, query("x.example."), "pol"); err != nil {
				t.Fatal(err)
			}
			if len(rt.Health.byID) != 1 || len(rt.pick.rr) != 1 {
				t.Fatal("old snapshot restored historical state")
			}
		})
	}
}

func TestRetargetedUpstreamDoesNotInheritOldEndpointFailure(t *testing.T) {
	up := startFake(t)
	rt := NewRuntime(nil, nil, nil, func(ctx context.Context, network, endpoint string) (net.Conn, error) {
		if endpoint == "old:53" {
			return nil, errors.New("old endpoint unreachable")
		}
		return defaultDial(ctx, network, endpoint)
	})
	old := snapTwo(t, "old:53", up.UDPAddr(), model.FailoverSpec{OnTransportError: true})
	old.Generation = 1
	for i := 0; i < 2; i++ {
		if _, err := rt.Exchange(t.Context(), old, query("x.example."), "pol"); err != nil {
			t.Fatal(err)
		}
	}
	if rt.Health.Healthy("bad") {
		t.Fatal("old endpoint was not marked down")
	}
	current := snapTwo(t, up.UDPAddr(), up.UDPAddr(), model.FailoverSpec{})
	current.Generation = 2
	current.Forwarding.Pools["pool"].Strategy = model.StrategyHealthAware
	result, err := rt.Exchange(t.Context(), current, query("x.example."), "pol")
	if err != nil {
		t.Fatal(err)
	}
	if result.UpstreamID != "bad" {
		t.Fatalf("new endpoint deprioritized by old endpoint health: %s", result.UpstreamID)
	}
}

func TestCallerCancellationDoesNotPoisonUpstreamHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	rt := NewRuntime(nil, nil, nil, func(ctx context.Context, _, _ string) (net.Conn, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	snap := churnSnapshot(1, "current", "127.0.0.1:1")
	done := make(chan error, 1)
	go func() { _, err := rt.Exchange(ctx, snap, query("x.example."), "pol"); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, failures := rt.Health.Snapshot("current"); failures != 0 {
		t.Fatalf("caller cancellation marked upstream unreachable: %d", failures)
	}
}

func TestTopologyPrunesRemovedStateAndRetainsCurrentHealth(t *testing.T) {
	rt := NewRuntime(nil, nil, nil, nil)
	first := churnSnapshot(1, "kept", "127.0.0.1:1")
	rt.observeTopology(first)
	rt.Health.RecordFailure("kept")
	rt.Health.RecordFailure("kept")
	rt.orderForSnapshot(first, first.Forwarding.Pools["pool-kept"])
	next := churnSnapshot(2, "kept", "127.0.0.1:1")
	rt.observeTopology(next)
	if down, failures := rt.Health.Snapshot("kept"); !down || failures != 2 {
		t.Fatalf("unchanged endpoint health discarded: down=%v failures=%d", down, failures)
	}
	if rt.pick.rr["pool-kept"].Load() != 1 {
		t.Fatal("unchanged round-robin counter discarded")
	}
	transport := churnSnapshot(3, "kept", "127.0.0.1:1")
	transport.Forwarding.Pools["pool-kept"].Upstreams[0].Transport = model.TransportTCP
	rt.observeTopology(transport)
	if down, failures := rt.Health.Snapshot("kept"); down || failures != 0 {
		t.Fatal("transport retarget retained previous health")
	}
	empty := &snapshot.Snapshot{Generation: 4}
	rt.observeTopology(empty)
	if len(rt.Health.byID) != 0 || len(rt.pick.rr) != 0 || len(rt.topologyUpstreams) != 0 {
		t.Fatal("empty topology retained forwarding runtime state")
	}
}

func TestTopologyRuntimeIgnoresSyntheticHealthFailures(t *testing.T) {
	for _, opts := range []ExchangeOpts{{ForceTimeout: true}, {ForceTransportError: true}} {
		rt := NewRuntime(nil, nil, nil, nil)
		snap := churnSnapshot(1, "current", "127.0.0.1:1")
		rt.observeTopology(snap)
		rt.Health.RecordFailure("current")
		if _, err := rt.ExchangeOpts(t.Context(), snap, query("x.example."), "pol", opts); err != nil {
			t.Fatal(err)
		}
		if down, failures := rt.Health.Snapshot("current"); down || failures != 1 {
			t.Fatalf("synthetic failure changed reachability: down=%v failures=%d", down, failures)
		}
	}
}

func TestAttemptTimeoutStillRecordsReachabilityFailure(t *testing.T) {
	rt := NewRuntime(nil, nil, nil, func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	snap := churnSnapshot(1, "current", "127.0.0.1:1")
	snap.Forwarding.ByID["pol"].Failover.Timeout = time.Millisecond
	result, err := rt.Exchange(t.Context(), snap, query("x.example."), "pol")
	if err != nil || result.RCode != model.RCodeServFail {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, failures := rt.Health.Snapshot("current"); failures != 1 {
		t.Fatalf("real attempt timeout did not count: %d", failures)
	}
}
