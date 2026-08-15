package forwarder

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestUDPSuccessAndTCPFallback(t *testing.T) {
	up := startFake(t)
	up.setAnswers(model.RR{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: 30 * time.Second, Data: "192.0.2.9"})
	snap := snapOne(t, up.UDPAddr(), model.TransportUDP, model.FailoverSpec{UDPTruncateRetryTCP: true})
	rt := NewRuntime(nil, nil, nil, nil)

	res, err := rt.Exchange(t.Context(), snap, query("a.example."), "pol")
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeNoError || len(res.Answers) != 1 || res.Answers[0].Data != "192.0.2.9" {
		t.Fatalf("%+v", res)
	}
	if res.AA || res.AD || res.RA {
		t.Fatalf("forwarded flags AA=%v AD=%v RA=%v", res.AA, res.AD, res.RA)
	}
	if res.Source != model.SourceUpstream || res.UpstreamID != "u1" {
		t.Fatalf("meta %+v", res)
	}

	up.setTruncate(true)
	up.setAnswers(model.RR{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: 30 * time.Second, Data: "192.0.2.9"})
	before := up.Packets.Load()
	res, err = rt.Exchange(t.Context(), snap, query("a.example."), "pol")
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeNoError {
		t.Fatalf("tcp retry rcode=%s", res.RCode)
	}
	if up.Packets.Load() <= before {
		t.Fatal("expected extra packets for TCP retry")
	}
}

func TestFailoverMatrix(t *testing.T) {
	bad := startFake(t)
	good := startFake(t)
	good.setAnswers(model.RR{Name: "x.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.10"})

	type tc struct {
		name    string
		badCode model.RCode
		hang    bool
		fo      model.FailoverSpec
		want    model.RCode
		wantUP  model.UpstreamID
	}
	cases := []tc{
		{name: "nxdomain-no-failover", badCode: model.RCodeNXDomain, fo: model.FailoverSpec{OnSERVFAIL: true, OnREFUSED: true, OnTimeout: true, OnTransportError: true}, want: model.RCodeNXDomain, wantUP: "bad"},
		{name: "servfail-failover", badCode: model.RCodeServFail, fo: model.FailoverSpec{OnSERVFAIL: true}, want: model.RCodeNoError, wantUP: "good"},
		{name: "servfail-no-failover", badCode: model.RCodeServFail, fo: model.FailoverSpec{}, want: model.RCodeServFail, wantUP: "bad"},
		{name: "refused-failover", badCode: model.RCodeRefused, fo: model.FailoverSpec{OnREFUSED: true}, want: model.RCodeNoError, wantUP: "good"},
		{name: "refused-no-failover", badCode: model.RCodeRefused, fo: model.FailoverSpec{}, want: model.RCodeRefused, wantUP: "bad"},
		{name: "timeout-failover", hang: true, fo: model.FailoverSpec{Timeout: 50 * time.Millisecond, OnTimeout: true}, want: model.RCodeNoError, wantUP: "good"},
		{name: "timeout-no-failover", hang: true, fo: model.FailoverSpec{Timeout: 50 * time.Millisecond}, want: model.RCodeServFail, wantUP: "bad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad.setRCode(tc.badCode)
			bad.setHang(tc.hang)
			if tc.badCode == model.RCodeNXDomain {
				bad.setAuth(model.RR{Name: "example.", Type: model.TypeSOA, Class: model.ClassIN, TTL: time.Second, Data: "ns.example. hostmaster. 1 1 1 1 1"})
			}
			snap := snapTwo(t, bad.UDPAddr(), good.UDPAddr(), tc.fo)
			rt := NewRuntime(nil, nil, NewHealth(nil), nil)
			ctx := t.Context()
			if tc.hang {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
			}
			res, err := rt.Exchange(ctx, snap, query("x.example."), "pol")
			if err != nil {
				t.Fatal(err)
			}
			if res.RCode != tc.want {
				t.Fatalf("rcode=%s want %s", res.RCode, tc.want)
			}
			if res.UpstreamID != tc.wantUP {
				t.Fatalf("up=%s want %s", res.UpstreamID, tc.wantUP)
			}
		})
	}
}

func TestTransportErrorFailover(t *testing.T) {
	good := startFake(t)
	good.setAnswers(model.RR{Name: "x.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.11"})
	fo := model.FailoverSpec{OnTransportError: true, Timeout: 100 * time.Millisecond}
	snap := snapTwo(t, "127.0.0.1:1", good.UDPAddr(), fo)
	res, err := NewRuntime(nil, nil, nil, nil).Exchange(t.Context(), snap, query("x.example."), "pol")
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeNoError || res.UpstreamID != "good" {
		t.Fatalf("%+v", res)
	}
	fo.OnTransportError = false
	snap = snapTwo(t, "127.0.0.1:1", good.UDPAddr(), fo)
	res, err = NewRuntime(nil, nil, nil, nil).Exchange(t.Context(), snap, query("x.example."), "pol")
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeServFail || res.UpstreamID != "bad" {
		t.Fatalf("no failover: %+v", res)
	}
}

func TestCDPassThroughAndNoAD(t *testing.T) {
	up := startFake(t)
	up.mu.Lock()
	up.handler = func(q model.Query) model.Result {
		return model.Result{RCode: model.RCodeNoError, CD: q.CD, Answers: []model.RR{{
			Name: q.Name, Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.1",
		}}}
	}
	up.mu.Unlock()
	snap := snapOne(t, up.UDPAddr(), model.TransportUDP, model.FailoverSpec{})
	q := query("cd.example.")
	q.CD = true
	res, err := NewRuntime(nil, nil, nil, nil).Exchange(t.Context(), snap, q, "pol")
	if err != nil {
		t.Fatal(err)
	}
	if !res.CD {
		t.Fatal("CD must pass through on forwarded answers")
	}
	if res.AD || res.AA {
		t.Fatal("must not forge AD or set AA")
	}
}

func TestCancelDoesNotLeak(t *testing.T) {
	up := startFake(t)
	up.setHang(true)
	snap := snapOne(t, up.UDPAddr(), model.TransportUDP, model.FailoverSpec{Timeout: time.Second})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := NewRuntime(nil, nil, nil, nil).Exchange(ctx, snap, query("x.example."), "pol")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestUnknownPolicyAndNilSnapshot(t *testing.T) {
	_, err := Exchange(t.Context(), nil, query("x."), "p")
	if !errors.Is(err, ErrNilSnapshot) {
		t.Fatalf("err=%v", err)
	}
	snap := &snapshot.Snapshot{Forwarding: snapshot.ForwardingIndex{ByID: map[model.PolicyID]*snapshot.CompiledPolicy{}}}
	_, err = Exchange(t.Context(), snap, query("x."), "missing")
	if !errors.Is(err, ErrUnknownPolicy) {
		t.Fatalf("err=%v", err)
	}
}

func TestChaosExchangeHooks(t *testing.T) {
	up := startFake(t)
	up.setAnswers(model.RR{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.9"})
	snap := snapOne(t, up.UDPAddr(), model.TransportUDP, model.FailoverSpec{OnTimeout: true, OnTransportError: true})
	rt := NewRuntime(nil, nil, nil, nil)
	res, err := rt.ExchangeOpts(t.Context(), snap, query("a.example."), "pol", ExchangeOpts{SyntheticRCode: model.RCodeRefused})
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeRefused {
		t.Fatalf("synthetic %s", res.RCode)
	}
	before := up.Packets.Load()
	res, err = rt.ExchangeOpts(t.Context(), snap, query("a.example."), "pol", ExchangeOpts{ForceTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeServFail {
		t.Fatalf("timeout rcode=%s", res.RCode)
	}
	if up.Packets.Load() != before {
		t.Fatal("forced timeout must not dial")
	}
	res, err = rt.ExchangeOpts(t.Context(), snap, query("a.example."), "pol", ExchangeOpts{Unavailable: map[model.UpstreamID]bool{"u1": true}})
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeServFail {
		t.Fatalf("unavailable rcode=%s", res.RCode)
	}
}

func query(name string) model.Query {
	return model.Query{Name: model.Name(name), Type: model.TypeA, Class: model.ClassIN, RD: true}
}

func snapOne(t *testing.T, endpoint string, tr model.Transport, fo model.FailoverSpec) *snapshot.Snapshot {
	t.Helper()
	st := &model.State{Spec: model.Spec{Forwarding: model.ForwardingSpec{
		Pools: []model.UpstreamPool{{
			ID: "pool", Strategy: model.StrategyOrdered,
			Upstreams: []model.Upstream{{ID: "u1", Endpoint: endpoint, Transport: tr}},
		}},
		Policies: []model.ForwardingPolicy{{ID: "pol", Suffix: ".", UpstreamPool: "pool", Failover: fo}},
	}}}
	idx, err := Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	return &snapshot.Snapshot{Forwarding: idx, Revision: "sha256:t"}
}

func snapTwo(t *testing.T, badEP, goodEP string, fo model.FailoverSpec) *snapshot.Snapshot {
	t.Helper()
	st := &model.State{Spec: model.Spec{Forwarding: model.ForwardingSpec{
		Pools: []model.UpstreamPool{{
			ID: "pool", Strategy: model.StrategyOrdered,
			Upstreams: []model.Upstream{
				{ID: "bad", Endpoint: badEP, Transport: model.TransportUDP},
				{ID: "good", Endpoint: goodEP, Transport: model.TransportUDP},
			},
		}},
		Policies: []model.ForwardingPolicy{{ID: "pol", Suffix: ".", UpstreamPool: "pool", Failover: fo}},
	}}}
	idx, err := Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	return &snapshot.Snapshot{Forwarding: idx, Revision: "sha256:t"}
}

func TestHealthAwareDeterministic(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	h := NewHealth(clk)
	h.fails = 2
	h.cool = 30 * time.Second
	h.RecordFailure("u1")
	if down, _ := h.Snapshot("u1"); down {
		t.Fatal("not down after 1 fail")
	}
	h.RecordFailure("u1")
	if down, _ := h.Snapshot("u1"); !down {
		t.Fatal("down after threshold")
	}
	if h.Healthy("u1") {
		t.Fatal("should skip until cooldown")
	}
	clk.Advance(30 * time.Second)
	if !h.Healthy("u1") {
		t.Fatal("cooldown expired is probe-eligible")
	}
	h.RecordSuccess("u1")
	if down, fails := h.Snapshot("u1"); down || fails != 0 {
		t.Fatalf("success reset down=%v fails=%d", down, fails)
	}

	pool := &snapshot.CompiledPool{
		ID:       "p",
		Strategy: model.StrategyHealthAware,
		Upstreams: []snapshot.CompiledUpstream{
			{ID: "u1", Endpoint: "10.0.0.1:53", Transport: model.TransportUDP},
			{ID: "u2", Endpoint: "10.0.0.2:53", Transport: model.TransportUDP},
		},
	}
	h.RecordFailure("u1")
	h.RecordFailure("u1")
	pk := newPicker(testutil.NewSeededRand(1), h)
	order := pk.order(pool)
	if order[0].ID != "u2" {
		t.Fatalf("health-aware should prefer healthy first, got %s", order[0].ID)
	}
}

func TestStrategiesOrderedRRRandom(t *testing.T) {
	h := NewHealth(nil)
	h.RecordFailure("a")
	h.RecordFailure("a")
	if down, _ := h.Snapshot("a"); !down {
		t.Fatal("need a down so production Health path is exercised")
	}
	pool := &snapshot.CompiledPool{
		ID: "p", Strategy: model.StrategyOrdered,
		Upstreams: []snapshot.CompiledUpstream{
			{ID: "a"}, {ID: "b"}, {ID: "c"},
		},
	}
	pk := newPicker(testutil.NewSeededRand(42), h)
	if got := pk.order(pool)[0].ID; got != "a" {
		t.Fatalf("ordered must start at configured 0 even when down, got %s", got)
	}
	pool.Strategy = model.StrategyRoundRobin
	var first []model.UpstreamID
	for i := 0; i < 3; i++ {
		first = append(first, pk.order(pool)[0].ID)
	}
	if first[0] == first[1] && first[1] == first[2] {
		t.Fatalf("round-robin did not rotate: %v", first)
	}
	pool.Strategy = model.StrategyRandom
	_ = pk.order(pool)
	pool.Strategy = model.StrategyHealthAware
	if got := pk.order(pool)[0].ID; got != "b" {
		t.Fatalf("health-aware should skip down a, got %s", got)
	}
}

func TestDefaultTimeoutFailoverFitsQueryBudget(t *testing.T) {
	bad := startFake(t)
	good := startFake(t)
	bad.setHang(true)
	good.setAnswers(model.RR{Name: "x.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.12"})
	// Timeout omitted (0) → DefaultExchangeTimeout 500ms. Parent 2s matches
	// the query-handler budget and must still allow a second try.
	fo := model.FailoverSpec{OnTimeout: true}
	snap := snapTwo(t, bad.UDPAddr(), good.UDPAddr(), fo)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	res, err := NewRuntime(nil, nil, nil, nil).Exchange(ctx, snap, query("x.example."), "pol")
	if err != nil {
		t.Fatal(err)
	}
	if res.RCode != model.RCodeNoError || res.UpstreamID != "good" {
		t.Fatalf("%+v", res)
	}
}
