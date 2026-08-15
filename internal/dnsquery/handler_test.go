package dnsquery

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/resolver"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestPackSampleLocalFromLoopback(t *testing.T) {
	st := loadPack(t)
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := New(store, nil, nil, nil, nil)

	res := serve(t, h, model.Query{
		Name:      "ns1.lab.example.net.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("127.0.0.1"),
		Transport: model.TransportUDP,
		RD:        true,
	})
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if res.RA {
		t.Fatal("unknown client (127.0.0.1 not in pack-sample groups) must have RA=0")
	}
	if !res.AA {
		t.Fatal("authoritative local hit must set AA")
	}
	wantRR(t, res, model.TypeA, "10.42.0.53")
}

func TestUnmatchedForwardOnlyIsRefusedZeroPackets(t *testing.T) {
	st := loadPack(t)
	up := startQueryFake(t)
	rewriteUpstreams(st, up.UDPAddr())
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store, Fwd: forwarder.NewRuntime(nil, nil, nil, nil)})

	before := up.Packets.Load()
	res := serve(t, h, model.Query{
		Name:      "www.unmatched.example.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("203.0.113.9"),
		Transport: model.TransportUDP,
		RD:        true,
	})
	if res.RCode != model.RCodeRefused {
		t.Fatalf("rcode=%s want REFUSED", res.RCode)
	}
	if res.RA {
		t.Fatal("RA must be 0")
	}
	if up.Packets.Load() != before {
		t.Fatalf("upstream packets=%d, want 0", up.Packets.Load()-before)
	}
	if h.DeniedForward() < 1 {
		t.Fatal("denied_forward must increment")
	}
}

func TestKnownGroupForwards(t *testing.T) {
	st := loadPack(t)
	up := startQueryFake(t)
	up.setAnswers(model.RR{Name: "intranet.corp.example.net.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "10.9.9.9"})
	rewriteUpstreams(st, up.UDPAddr())
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	c := cache.New(cache.PolicyFromSpec(st.Spec.Cache), nil)
	h := NewOpts(Opts{Store: store, Cache: c, Fwd: forwarder.NewRuntime(nil, nil, nil, nil)})

	res := serve(t, h, model.Query{
		Name:      "intranet.corp.example.net.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("10.42.0.10"),
		Transport: model.TransportUDP,
		RD:        true,
	})
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if !res.RA {
		t.Fatal("known AllowForward group with a policy must set RA")
	}
	if res.AA {
		t.Fatal("forwarded answers are not AA")
	}
	wantRR(t, res, model.TypeA, "10.9.9.9")
	if up.Packets.Load() < 1 {
		t.Fatal("expected upstream packet")
	}

	// cache hit: no extra packet
	before := up.Packets.Load()
	res2 := serve(t, h, model.Query{
		Name:      "intranet.corp.example.net.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("10.42.0.10"),
		Transport: model.TransportUDP,
		RD:        true,
	})
	if res2.Source != model.SourceCache {
		t.Fatalf("source=%s", res2.Source)
	}
	if up.Packets.Load() != before {
		t.Fatal("cache hit must not dial")
	}
}

func TestLocalOnlyGroupNeverForwards(t *testing.T) {
	st := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "t"},
		Spec: model.Spec{
			Access: model.AccessSpec{
				UnknownClient: model.UnknownClientRefuseForward,
				ClientGroups: []model.ClientGroup{{
					ID:           "local",
					CIDRs:        []string{"192.0.2.0/24"},
					AllowForward: false,
				}},
			},
			Defaults: model.DefaultsSpec{TTL: 30 * time.Second, NegativeTTL: 10 * time.Second, CNAMEDepth: 8},
			Zones: []model.Zone{{
				ID:   "z",
				Name: "lab.example.",
				Mode: model.ZoneModeAuthoritative,
				SOA:  &model.SOA{Primary: "ns.lab.example.", Administrator: "hostmaster.lab.example.", Serial: "1", Refresh: time.Hour, Retry: time.Minute, Expire: time.Hour, Minimum: 10 * time.Second},
				Records: []model.Record{{
					ID: "a", Owner: "ns", Type: model.TypeA, TTL: time.Second, Values: []string{"192.0.2.53"},
				}},
			}},
			Forwarding: model.ForwardingSpec{
				Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
				Pools: []model.UpstreamPool{{
					ID: "p", Strategy: model.StrategyOrdered,
					Upstreams: []model.Upstream{{ID: "u", Endpoint: "127.0.0.1:9", Transport: model.TransportUDP}},
				}},
			},
		},
	}
	up := startQueryFake(t)
	st.Spec.Forwarding.Pools[0].Upstreams[0].Endpoint = up.UDPAddr()
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})

	local := serve(t, h, model.Query{
		Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("192.0.2.8"), RD: true,
	})
	if local.RCode != model.RCodeNoError || local.RA {
		t.Fatalf("local-only local name: %+v", local)
	}
	before := up.Packets.Load()
	ref := serve(t, h, model.Query{
		Name: "other.example.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("192.0.2.8"), RD: true,
	})
	if ref.RCode != model.RCodeRefused || ref.RA {
		t.Fatalf("local-only forward-only: %+v", ref)
	}
	if up.Packets.Load() != before {
		t.Fatal("local-only group must not hit upstream")
	}
}

func TestFlagMatrix(t *testing.T) {
	up := startQueryFake(t)
	up.setAnswers(model.RR{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.77"})
	st := &model.State{Spec: model.Spec{
		Access: model.AccessSpec{UnknownClient: model.UnknownClientRefuseForward, ClientGroups: []model.ClientGroup{
			{ID: "fwd", CIDRs: []string{"10.1.0.0/16"}, AllowForward: true},
			{ID: "loc", CIDRs: []string{"10.2.0.0/16"}, AllowForward: false},
		}},
		Defaults: model.DefaultsSpec{TTL: 30 * time.Second, NegativeTTL: 10 * time.Second, CNAMEDepth: 8},
		Zones: []model.Zone{
			{
				ID: "auth", Name: "auth.example.", Mode: model.ZoneModeAuthoritative,
				SOA:     &model.SOA{Primary: "ns.auth.example.", Administrator: "h.auth.example.", Serial: "1", Refresh: time.Hour, Retry: time.Minute, Expire: time.Hour, Minimum: time.Second},
				Records: []model.Record{{ID: "a", Owner: "ns", Type: model.TypeA, TTL: time.Second, Values: []string{"10.0.0.1"}}},
			},
			{
				ID: "ov", Name: "ov.example.", Mode: model.ZoneModeOverlay,
				Records: []model.Record{{ID: "o", Owner: "hit", Type: model.TypeA, TTL: time.Second, Values: []string{"10.0.0.2"}}},
			},
		},
		Forwarding: model.ForwardingSpec{
			Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
			Pools: []model.UpstreamPool{{ID: "p", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{
				{ID: "u", Endpoint: up.UDPAddr(), Transport: model.TransportUDP},
			}}},
		},
	}}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})

	type tc struct {
		name           string
		qname          string
		client         string
		rd, cd         bool
		wantAA, wantRA bool
		wantCD         bool
		rcode          model.RCode
	}
	fwd := "10.1.0.8"
	loc := "10.2.0.8"
	unk := "203.0.113.1"
	cases := []tc{
		{name: "auth-fwd-RD1-CD1", qname: "ns.auth.example.", client: fwd, rd: true, cd: true, wantAA: true, wantRA: true, rcode: model.RCodeNoError},
		{name: "auth-unk-RD1-CD1", qname: "ns.auth.example.", client: unk, rd: true, cd: true, wantAA: true, rcode: model.RCodeNoError},
		{name: "auth-loc-RD0-CD0", qname: "ns.auth.example.", client: loc, wantAA: true, rcode: model.RCodeNoError},
		{name: "overlay-hit-fwd", qname: "hit.ov.example.", client: fwd, rd: true, wantRA: true, rcode: model.RCodeNoError},
		{name: "overlay-miss-fwd", qname: "miss.ov.example.", client: fwd, rd: true, cd: true, wantRA: true, wantCD: true, rcode: model.RCodeNoError},
		{name: "overlay-miss-unk", qname: "miss.ov.example.", client: unk, rd: true, rcode: model.RCodeRefused},
		{name: "none-fwd-RD1", qname: "out.example.", client: fwd, rd: true, wantRA: true, rcode: model.RCodeNoError},
		{name: "none-fwd-RD0", qname: "out.example.", client: fwd, rcode: model.RCodeNoError, wantRA: true},
		{name: "none-unk-RD1", qname: "out.example.", client: unk, rd: true, rcode: model.RCodeRefused},
		{name: "none-loc-RD1", qname: "out.example.", client: loc, rd: true, rcode: model.RCodeRefused},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := serve(t, h, model.Query{
				Name: model.Name(tc.qname), Type: model.TypeA, Class: model.ClassIN,
				Client: netip.MustParseAddr(tc.client), RD: tc.rd, CD: tc.cd,
			})
			if res.RCode != tc.rcode {
				t.Fatalf("rcode=%s want %s", res.RCode, tc.rcode)
			}
			if res.AA != tc.wantAA {
				t.Fatalf("AA=%v want %v", res.AA, tc.wantAA)
			}
			if res.RA != tc.wantRA {
				t.Fatalf("RA=%v want %v", res.RA, tc.wantRA)
			}
			if res.AD {
				t.Fatal("AD must never be set")
			}
			if res.CD != tc.wantCD {
				t.Fatalf("CD=%v want %v", res.CD, tc.wantCD)
			}
		})
	}
}

func TestAuthoritativeMissNeverForwards(t *testing.T) {
	up := startQueryFake(t)
	st := &model.State{Spec: model.Spec{
		Access: model.AccessSpec{ClientGroups: []model.ClientGroup{
			{ID: "fwd", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true},
		}},
		Defaults: model.DefaultsSpec{TTL: time.Second, NegativeTTL: time.Second, CNAMEDepth: 8},
		Zones: []model.Zone{{
			ID: "z", Name: "lab.example.", Mode: model.ZoneModeAuthoritative,
			SOA:     &model.SOA{Primary: "ns.lab.example.", Administrator: "h.lab.example.", Serial: "1", Refresh: time.Hour, Retry: time.Minute, Expire: time.Hour, Minimum: time.Second},
			Records: []model.Record{{ID: "a", Owner: "ns", Type: model.TypeA, TTL: time.Second, Values: []string{"10.0.0.1"}}},
		}},
		Forwarding: model.ForwardingSpec{
			Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
			Pools:    []model.UpstreamPool{{ID: "p", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{{ID: "u", Endpoint: up.UDPAddr(), Transport: model.TransportUDP}}}},
		},
	}}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})
	before := up.Packets.Load()
	res := serve(t, h, model.Query{
		Name: "missing.lab.example.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("10.1.2.3"), RD: true,
	})
	if res.RCode != model.RCodeNXDomain || !res.AA {
		t.Fatalf("%+v", res)
	}
	if up.Packets.Load() != before {
		t.Fatal("authoritative miss must not forward")
	}
}

func TestOverlayCNAMEFallthroughForwardsTarget(t *testing.T) {
	up := startQueryFake(t)
	up.setAnswers(model.RR{Name: "outside.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.50"})
	st := &model.State{Spec: model.Spec{
		Access:   model.AccessSpec{ClientGroups: []model.ClientGroup{{ID: "fwd", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true}}},
		Defaults: model.DefaultsSpec{TTL: time.Second, NegativeTTL: time.Second, CNAMEDepth: 8},
		Zones: []model.Zone{{
			ID: "ov", Name: "ov.example.", Mode: model.ZoneModeOverlay,
			Records: []model.Record{{ID: "c", Owner: "alias", Type: model.TypeCNAME, TTL: time.Second, Values: []string{"outside.example."}}},
		}},
		Forwarding: model.ForwardingSpec{
			Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
			Pools:    []model.UpstreamPool{{ID: "p", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{{ID: "u", Endpoint: up.UDPAddr(), Transport: model.TransportUDP}}}},
		},
	}}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})
	res := serve(t, h, model.Query{
		Name: "alias.ov.example.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("10.0.0.9"), RD: true,
	})
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if len(res.Answers) < 2 {
		t.Fatalf("want CNAME+A, got %+v", res.Answers)
	}
	if res.Answers[0].Type != model.TypeCNAME {
		t.Fatalf("first rr %s", res.Answers[0].Type)
	}
	wantRR(t, res, model.TypeA, "192.0.2.50")
}

func TestOverlayCNAMEUsesTargetSuffixPolicy(t *testing.T) {
	corp := startQueryFake(t)
	def := startQueryFake(t)
	corp.setAnswers(model.RR{Name: "host.corp.example.net.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "10.0.0.53"})
	def.setAnswers(model.RR{Name: "host.corp.example.net.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "10.9.9.9"})
	st := &model.State{Spec: model.Spec{
		Access:   model.AccessSpec{ClientGroups: []model.ClientGroup{{ID: "fwd", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true}}},
		Defaults: model.DefaultsSpec{TTL: time.Second, NegativeTTL: time.Second, CNAMEDepth: 8},
		Zones: []model.Zone{{
			ID: "ov", Name: "ov.example.", Mode: model.ZoneModeOverlay,
			Records: []model.Record{{ID: "c", Owner: "alias", Type: model.TypeCNAME, TTL: time.Second, Values: []string{"host.corp.example.net."}}},
		}},
		Forwarding: model.ForwardingSpec{
			Policies: []model.ForwardingPolicy{
				{ID: "corp", Suffix: "corp.example.net.", UpstreamPool: "corporate"},
				{ID: "def", Suffix: ".", UpstreamPool: "default"},
			},
			Pools: []model.UpstreamPool{
				{ID: "corporate", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{
					{ID: "corp-1", Endpoint: corp.UDPAddr(), Transport: model.TransportUDP},
				}},
				{ID: "default", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{
					{ID: "def-1", Endpoint: def.UDPAddr(), Transport: model.TransportUDP},
				}},
			},
		},
	}}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})
	res := serve(t, h, model.Query{
		Name: "alias.ov.example.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("10.0.0.9"), RD: true,
	})
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	wantRR(t, res, model.TypeA, "10.0.0.53")
	if corp.Packets.Load() < 1 {
		t.Fatal("overlay CNAME target under corp.example.net. must dial the corporate pool")
	}
	if def.Packets.Load() != 0 {
		t.Fatalf("default pool packets=%d, want 0", def.Packets.Load())
	}
	if res.ForwardingID != "corp" {
		t.Fatalf("exchange policy=%s, want corp", res.ForwardingID)
	}
	if res.Explanation == nil || res.Explanation.ForwardingID != "def" {
		t.Fatalf("classified policy on original QNAME should stay default, got %+v", res.Explanation)
	}
}

func TestEmptyClientGroupsServesLocalForwardsNone(t *testing.T) {
	st := &model.State{Spec: model.Spec{
		Access:   model.AccessSpec{UnknownClient: model.UnknownClientRefuseForward, ClientGroups: nil},
		Defaults: model.DefaultsSpec{TTL: time.Second, NegativeTTL: time.Second, CNAMEDepth: 8},
		Zones: []model.Zone{{
			ID: "z", Name: "lab.example.", Mode: model.ZoneModeAuthoritative,
			SOA:     &model.SOA{Primary: "ns.lab.example.", Administrator: "h.lab.example.", Serial: "1", Refresh: time.Hour, Retry: time.Minute, Expire: time.Hour, Minimum: time.Second},
			Records: []model.Record{{ID: "a", Owner: "ns", Type: model.TypeA, TTL: time.Second, Values: []string{"10.0.0.1"}}},
		}},
		Forwarding: model.ForwardingSpec{
			Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
			Pools:    []model.UpstreamPool{{ID: "p", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{{ID: "u", Endpoint: "127.0.0.1:9", Transport: model.TransportUDP}}}},
		},
	}}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})
	res := serve(t, h, model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("8.8.8.8")})
	if res.RCode != model.RCodeNoError || res.RA {
		t.Fatalf("%+v", res)
	}
	ref := serve(t, h, model.Query{Name: "other.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("8.8.8.8"), RD: true})
	if ref.RCode != model.RCodeRefused {
		t.Fatalf("rcode=%s", ref.RCode)
	}
}

func TestNilSnapshotSERVFAIL(t *testing.T) {
	h := New(snapshot.NewStore(), nil, nil, nil, nil)
	res := serve(t, h, model.Query{Name: "x.", Type: model.TypeA, Class: model.ClassIN})
	if res.RCode != model.RCodeServFail {
		t.Fatalf("rcode=%s", res.RCode)
	}
}

func serve(t *testing.T, h dnsserver.Handler, q model.Query) model.Result {
	t.Helper()
	resp, hint, err := h.ServeDNS(t.Context(), &q)
	if err != nil {
		t.Fatal(err)
	}
	if hint != dnsserver.HintSend || resp == nil {
		t.Fatalf("hint=%s resp=%v", hint, resp)
	}
	return resp.Result()
}

func wantRR(t *testing.T, res model.Result, typ model.RRType, data string) {
	t.Helper()
	for _, rr := range res.Answers {
		if rr.Type == typ && rr.Data == data {
			return
		}
	}
	t.Fatalf("missing %s %q in %+v", typ, data, res.Answers)
}

func loadPack(t *testing.T) *model.State {
	t.Helper()
	st, err := config.LoadFile(filepath.Join(repoRoot(t), "testdata/config/valid/pack-sample.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func compileSnap(t *testing.T, st *model.State) *snapshot.Snapshot {
	t.Helper()
	z, err := resolver.Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	f, err := forwarder.Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	acc, err := snapshot.CompileAccess(st)
	if err != nil {
		t.Fatal(err)
	}
	rev, err := config.Revision(st)
	if err != nil {
		rev = "sha256:test"
	}
	return &snapshot.Snapshot{
		Canonical:  st,
		Revision:   rev,
		Generation: 1,
		Access:     acc,
		Zones:      z,
		Forwarding: f,
		Defaults: snapshot.DefaultsView{
			TTL:         st.Spec.Defaults.TTL,
			NegativeTTL: st.Spec.Defaults.NegativeTTL,
			CNAMEDepth:  st.Spec.Defaults.CNAMEDepth,
		},
		CachePolicy: snapshot.CachePolicy{
			Enabled:            st.Spec.Cache.Enabled,
			MaxEntries:         st.Spec.Cache.MaxEntries,
			MinimumTTL:         st.Spec.Cache.MinimumTTL,
			MaximumTTL:         st.Spec.Cache.MaximumTTL,
			MaximumNegativeTTL: st.Spec.Cache.MaximumNegativeTTL,
			StaleServing:       st.Spec.Cache.StaleServing,
		},
	}
}

func rewriteUpstreams(st *model.State, endpoint string) {
	for i := range st.Spec.Forwarding.Pools {
		for j := range st.Spec.Forwarding.Pools[i].Upstreams {
			st.Spec.Forwarding.Pools[i].Upstreams[j].Endpoint = endpoint
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
