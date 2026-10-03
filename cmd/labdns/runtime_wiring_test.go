package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/observability"
)

func TestRuntimeReportsActualUpstreamFailure(t *testing.T) {
	path := ephemeralPackSample(t)
	state, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := ln.Addr().String()
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			connection, err := ln.Accept()
			if err != nil {
				return
			}
			_ = connection.Close()
		}
	}()
	state.Spec.Access.ClientGroups[0].CIDRs = []string{"127.0.0.0/8"}
	state.Spec.Forwarding.Pools = []model.UpstreamPool{{ID: "corporate", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{{ID: "corp-1", Endpoint: endpoint, Transport: model.TransportTCP}}}, {ID: "default", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{{ID: "broken", Endpoint: endpoint, Transport: model.TransportTCP}}}}
	body, err := config.CanonicalJSON(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: path})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()
	query, err := dnswire.PackQuery(1, model.Query{Name: "outside.invalid.", Type: model.TypeA, Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		exchangeUDPTest(t, rt.UDPAddr(), query)
	}
	status, err := rt.app.Status(t.Context(), auth.Actor{Role: auth.RoleAdministrator})
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Upstreams) != 2 || !status.Degraded {
		t.Fatalf("real upstream outage hidden: %+v", status)
	}
}

func TestRuntimeShutdownStopsSignalWorkerWithLiveParent(t *testing.T) {
	before := signalWorkerCount()
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: ephemeralPackSample(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for signalWorkerCount() > before && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if got := signalWorkerCount(); got > before {
		t.Fatalf("signal worker leaked after Shutdown with live parent: before=%d after=%d", before, got)
	}
}

func signalWorkerCount() int {
	buffer := make([]byte, 1<<20)
	size := runtime.Stack(buffer, true)
	return bytes.Count(buffer[:size], []byte("internal/chaos.ServeSignals("))
}

func TestRuntimeControlMetricsShareRegistry(t *testing.T) {
	rt, err := serveFromConfig(t.Context(), serveFlags{Config: ephemeralPackSample(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()
	response, err := http.Get("http://" + rt.MgmtAddr() + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	foundCalls, foundGeneration := false, false
	for _, sample := range rt.metrics.Snapshot() {
		if sample.Name == observability.MetricCapabilityCalls && sample.Labels["transport"] == "rest" && sample.Value == 1 {
			foundCalls = true
		}
		if sample.Name == observability.MetricStateGeneration {
			foundGeneration = true
		}
	}
	if !foundCalls || !foundGeneration {
		t.Fatalf("shared metrics absent: calls=%v generation=%v", foundCalls, foundGeneration)
	}
}

func TestRuntimeStartupFailureStopsSignalWorker(t *testing.T) {
	path := ephemeralPackSample(t)
	state, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state.Spec.Management.Auth.Profile = auth.ProfileBearer
	state.Spec.Management.Auth.SecretRef = filepath.Join(t.TempDir(), "missing-token")
	body, err := config.CanonicalJSON(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	before := signalWorkerCount()
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: path})
	if err == nil || rt != nil {
		t.Fatal("missing secret did not fail startup")
	}
	if got := signalWorkerCount(); got > before {
		t.Fatalf("early failure leaked signal worker: before=%d after=%d", before, got)
	}
}

func TestRuntimeCacheMutationAppliesOverUDPAndTCP(t *testing.T) {
	for _, transport := range []string{"udp", "tcp"} {
		t.Run(transport, func(t *testing.T) {
			rt, err := serveFromConfig(t.Context(), serveFlags{Config: ephemeralPackSample(t)})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rt.Shutdown(context.Background()) }()
			query, err := dnswire.PackQuery(1, model.Query{Name: "ns1.lab.example.net.", Type: model.TypeA, Class: model.ClassIN}, nil)
			if err != nil {
				t.Fatal(err)
			}
			exchange := func() *dnswire.UpstreamMsg {
				var raw []byte
				if transport == "udp" {
					raw = exchangeUDPTest(t, rt.UDPAddr(), query)
				} else {
					connection, err := net.Dial("tcp", rt.TCPAddr().String())
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = connection.Close() }()
					_ = connection.SetDeadline(time.Now().Add(time.Second))
					message := make([]byte, 2+len(query))
					binary.BigEndian.PutUint16(message, uint16(len(query)))
					copy(message[2:], query)
					if _, err := connection.Write(message); err != nil {
						t.Fatal(err)
					}
					prefix := make([]byte, 2)
					if _, err := io.ReadFull(connection, prefix); err != nil {
						t.Fatal(err)
					}
					raw = make([]byte, int(binary.BigEndian.Uint16(prefix)))
					if _, err := io.ReadFull(connection, raw); err != nil {
						t.Fatal(err)
					}
				}
				result, err := dnswire.UnpackUpstream(raw)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			exchange()
			exchange()
			cachePolicy := rt.Store().Load().Canonical.Spec.Cache
			cachePolicy.MaximumTTL = 2 * time.Second
			actor := auth.Actor{Role: auth.RoleAdministrator}
			apply := func() {
				value, err := json.Marshal(cachePolicy)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := rt.app.Apply(t.Context(), actor, app.ChangeIn{ExpectedRevision: rt.Store().Load().Revision, Operations: []model.Operation{{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetCache}, Value: value}}}); err != nil {
					t.Fatal(err)
				}
			}
			apply()
			exchange()
			cached := exchange()
			if len(cached.Answers) != 1 || cached.Answers[0].TTL > 2*time.Second {
				t.Fatalf("live snapshot TTL bound ignored: %+v", cached)
			}
			cachePolicy.Enabled = false
			apply()
			before, err := rt.app.CacheStatus(t.Context(), actor)
			if err != nil {
				t.Fatal(err)
			}
			exchange()
			exchange()
			after, err := rt.app.CacheStatus(t.Context(), actor)
			if err != nil {
				t.Fatal(err)
			}
			if after.Enabled || after.Entries != 0 || after.Hits != before.Hits {
				t.Fatalf("disabled cache served packets: before=%+v after=%+v", before, after)
			}
			if _, err := rt.app.Reset(t.Context(), actor, app.ResetIn{}); err != nil {
				t.Fatal(err)
			}
			exchange()
			exchange()
			restored, err := rt.app.CacheStatus(t.Context(), actor)
			if err != nil || !restored.Enabled || restored.Entries == 0 {
				t.Fatalf("reset cache not restored: %+v %v", restored, err)
			}
		})
	}
}
