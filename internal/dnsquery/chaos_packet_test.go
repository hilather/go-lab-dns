package dnsquery

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestPacketFixedDelayExactRecord(t *testing.T) {
	h, _ := chaosHandler(t, delayPolicy("slow", model.RecordID("a1"), model.PhaseBeforeResponse, 40*time.Millisecond, 0))
	q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
	start := time.Now()
	res := serve(t, h, q)
	elapsed := time.Since(start)
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if elapsed < 35*time.Millisecond {
		t.Fatalf("elapsed %s, want >= 40ms delay", elapsed)
	}
}

func TestPacketUniformDelayWildcardBounds(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{uniformPolicy("uw", "tools-wildcard-a", 20*time.Millisecond, 40*time.Millisecond)}
	h := handlerFromState(t, st, nil)
	q := model.Query{Name: "foo.tools.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
	start := time.Now()
	res := serve(t, h, q)
	elapsed := time.Since(start)
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if elapsed < 15*time.Millisecond || elapsed > 400*time.Millisecond {
		t.Fatalf("uniform delay out of bounds: %s", elapsed)
	}
}

func TestPacketRCodeAndEDE(t *testing.T) {
	for _, rc := range []string{"SERVFAIL", "REFUSED", "NXDOMAIN", "FORMERR", "NOTIMP"} {
		st := chaosState(t)
		st.Spec.Chaos.Policies = []model.ChaosPolicy{rcodePolicy("rc", rc, &model.EDE{Code: 0, Text: "lab-injected"})}
		if rc == "FORMERR" || rc == "NOTIMP" {
			st.Spec.Chaos.Policies[0].SafetyClass = model.SafetyClassMedium
		}
		h, srv := startChaosServer(t, st)
		_ = h
		q := packQuery(t, "ns.lab.example.", model.TypeA, true)
		out := exchangeUDP(t, srv.UDPAddr(), q)
		msg, err := dnswire.UnpackUpstream(out)
		if err != nil {
			t.Fatal(err)
		}
		if string(msg.RCode) != rc {
			t.Fatalf("%s: rcode=%s", rc, msg.RCode)
		}
		if rc == "SERVFAIL" && !wireHasEDE(t, out, 0, "lab-injected") {
			t.Fatal("SERVFAIL missing EDE option 15")
		}
	}

	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{rcodePolicy("nd", "NODATA", &model.EDE{Code: 22, Text: "lab-nodata"})}
	h, srv := startChaosServer(t, st)
	_ = h
	q := packQuery(t, "ns.lab.example.", model.TypeA, true)
	out := exchangeUDP(t, srv.UDPAddr(), q)
	msg, err := dnswire.UnpackUpstream(out)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode != model.RCodeNoError || len(msg.Answers) != 0 {
		t.Fatalf("NODATA %+v", msg)
	}
}

func TestPacketEDEPresent(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{rcodePolicy("sf", "SERVFAIL", &model.EDE{Code: 0, Text: "lab-injected-failure"})}
	_, srv := startChaosServer(t, st)
	q := packQuery(t, "ns.lab.example.", model.TypeA, true)
	out := exchangeUDP(t, srv.UDPAddr(), q)
	req, err := dnswire.Parse(out, model.TransportUDP, netip.Addr{})
	if err != nil && !dnswire.IsMalformed(err) {
		// Unpack as response via miekg through Encode tests; use UnpackUpstream + re-parse OPT.
	}
	_ = req
	msg, err := dnswire.UnpackUpstream(out)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode != model.RCodeServFail {
		t.Fatalf("rcode=%s", msg.RCode)
	}
	// EDE is in OPT; UnpackUpstream strips OPT. Parse the wire OPT via a dedicated encode round-trip check.
	if !wireHasEDE(t, out, 0, "lab-injected-failure") {
		t.Fatal("missing EDE option on the wire")
	}
}

func TestPacketUDPDropAndTruncateThenTCP(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{transportPolicy("drop", model.ActionDrop, []model.Transport{model.TransportUDP})}
	_, srv := startChaosServer(t, st)
	q := packQuery(t, "ns.lab.example.", model.TypeA, false)
	if out := exchangeUDPTimeout(t, srv.UDPAddr(), q, 200*time.Millisecond); out != nil {
		t.Fatalf("drop sent %d bytes", len(out))
	}

	st = chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{transportPolicy("tc", model.ActionTruncate, []model.Transport{model.TransportUDP})}
	_, srv = startChaosServer(t, st)
	out := exchangeUDP(t, srv.UDPAddr(), q)
	if !hasTCFlag(out) {
		t.Fatal("expected TC")
	}
	// TCP retry is not in the UDP-only scope, so the full answer is sent.
	tcpOut := exchangeTCPMsg(t, srv.TCPAddr(), q)
	msg, err := dnswire.UnpackUpstream(tcpOut)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode != model.RCodeNoError || len(msg.Answers) == 0 {
		t.Fatalf("tcp retry %+v", msg)
	}
}

func TestPacketTCPCloseResetHold(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{transportPolicy("cls", model.ActionTCPClose, []model.Transport{model.TransportTCP})}
	_, srv := startChaosServer(t, st)
	if _, err := exchangeTCPMaybe(t, srv.TCPAddr(), packQuery(t, "ns.lab.example.", model.TypeA, false), 400*time.Millisecond); err == nil {
		t.Fatal("tcp-close should not return a DNS message")
	}

	st = chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{transportPolicy("rst", model.ActionTCPReset, []model.Transport{model.TransportTCP})}
	_, srv = startChaosServer(t, st)
	c, err := net.Dial("tcp", srv.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	writeTCPFrame(t, c, packQuery(t, "ns.lab.example.", model.TypeA, false))
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	var hdr [2]byte
	_, err = io.ReadFull(c, hdr[:])
	if err == nil {
		t.Fatal("tcp-reset returned a length prefix")
	}

	st = chaosState(t)
	p := transportPolicy("hold", model.ActionDrop, []model.Transport{model.TransportTCP})
	p.Outcomes[0].Actions[0].Hold = 50 * time.Millisecond
	st.Spec.Chaos.Policies = []model.ChaosPolicy{p}
	_, srv = startChaosServer(t, st)
	start := time.Now()
	_, err = exchangeTCPMaybe(t, srv.TCPAddr(), packQuery(t, "ns.lab.example.", model.TypeA, false), time.Second)
	if err == nil {
		t.Fatal("hold-then-close should not return a DNS message")
	}
	if time.Since(start) < 30*time.Millisecond {
		t.Fatalf("hold returned too fast: %s", time.Since(start))
	}
}

func TestCacheAndUpstreamEffects(t *testing.T) {
	up := startQueryFake(t)
	up.setAnswers(model.RR{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.9"})
	st := chaosState(t)
	st.Spec.Cache = model.CacheSpec{Enabled: true, MaxEntries: 16, MinimumTTL: time.Second, MaximumTTL: time.Minute}
	st.Spec.Forwarding = model.ForwardingSpec{
		Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
		Pools: []model.UpstreamPool{{
			ID: "p", Strategy: model.StrategyOrdered,
			Upstreams: []model.Upstream{{ID: "u1", Endpoint: up.UDPAddr(), Transport: model.TransportUDP}},
		}},
	}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "cb", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{Owners: []model.Name{"out.example."}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "c", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionCache, Value: chaos.CacheValueBypass, Phase: model.PhaseBeforeResolution},
		}}},
	}}
	c := cache.New(cache.PolicyFromSpec(st.Spec.Cache), nil)
	h := handlerFromState(t, st, c)
	q := model.Query{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP, RD: true}
	_ = serve(t, h, q)
	before := up.Packets.Load()
	_ = serve(t, h, q)
	if up.Packets.Load() <= before {
		t.Fatal("cache bypass must dial again")
	}

	st.Spec.Chaos.Policies[0].Outcomes[0].Actions = []model.ChaosAction{
		{Type: model.ActionUpstream, Value: "SERVFAIL", Phase: model.PhaseBeforeUpstream},
	}
	h = handlerFromState(t, st, nil)
	res := serve(t, h, q)
	if res.RCode != model.RCodeServFail {
		t.Fatalf("synthetic upstream rcode=%s", res.RCode)
	}
}

func TestEmergencyDisableUnderDelayedLoad(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	st := chaosState(t)
	st.Spec.Chaos.Safety.MaxConcurrentDelayed = 200
	st.Spec.Chaos.Policies = []model.ChaosPolicy{delayPolicy("slow", "a1", model.PhaseBeforeResponse, time.Hour, 0)}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	eng := chaos.NewEngine(clk, nil)
	h := NewOpts(Opts{Store: store, Engine: eng, Clock: clk})

	const n = 24
	type outcome struct {
		err  error
		hint dnsserver.TransportHint
		rc   model.RCode
	}
	got := make(chan outcome, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
			resp, hint, err := h.ServeDNS(context.Background(), &q)
			rc := model.RCode("")
			if resp != nil {
				rc = resp.Result().RCode
			}
			got <- outcome{err: err, hint: hint, rc: rc}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for eng.Budgets().InFlight() < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if eng.Budgets().InFlight() == 0 {
		t.Fatal("expected delayed in-flight queries")
	}
	chaos.EmergencyDisable(store, eng)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emergency disable did not cancel delayed load")
	}
	close(got)
	for o := range got {
		if o.err != nil {
			t.Fatalf("in-flight query failed after emergency: %v", o.err)
		}
		if o.hint != dnsserver.HintSend || o.rc != model.RCodeNoError {
			t.Fatalf("emergency must send the base answer, hint=%s rcode=%s", o.hint, o.rc)
		}
	}

	q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
	start := time.Now()
	res, hint, err := h.ServeDNS(context.Background(), &q)
	if err != nil || hint != dnsserver.HintSend || res.Result().RCode != model.RCodeNoError {
		t.Fatalf("post-disable %+v hint=%s err=%v", res, hint, err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("post-disable still delayed: %s", time.Since(start))
	}
}

func TestQueryCancelReleasesBudget(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{delayPolicy("slow", "a1", model.PhaseBeforeResponse, time.Hour, 0)}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	eng := chaos.NewEngine(clk, nil)
	h := NewOpts(Opts{Store: store, Engine: eng, Clock: clk})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP}
		_, _, err := h.ServeDNS(ctx, &q)
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for eng.Budgets().InFlight() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("query did not return")
	}
	if eng.Budgets().InFlight() != 0 {
		t.Fatalf("leaked %d", eng.Budgets().InFlight())
	}
}

func TestEmergencyDisableDuringUpstreamNoPostChaos(t *testing.T) {
	up := startQueryFake(t)
	up.setAnswers(model.RR{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, TTL: 30 * time.Second, Data: "192.0.2.9"})
	hold := make(chan struct{})
	up.setHold(hold)
	st := chaosState(t)
	st.Spec.Forwarding = model.ForwardingSpec{
		Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
		Pools: []model.UpstreamPool{{
			ID: "p", Strategy: model.StrategyOrdered,
			Upstreams: []model.Upstream{{ID: "u1", Endpoint: up.UDPAddr(), Transport: model.TransportUDP}},
		}},
	}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "mut", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{Owners: []model.Name{"out.example."}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "m", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionRCode, Value: "SERVFAIL", Phase: model.PhaseBeforeResponse},
			{Type: model.ActionTTL, Value: chaos.TTLValueZero, Phase: model.PhaseBeforeResponse},
		}}},
	}}
	h, store, eng := buildChaos(t, st, nil)
	done := make(chan struct {
		res  model.Result
		hint dnsserver.TransportHint
		err  error
	}, 1)
	go func() {
		q := model.Query{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP, RD: true}
		resp, hint, err := h.ServeDNS(context.Background(), &q)
		out := model.Result{}
		if resp != nil {
			out = resp.Result()
		}
		done <- struct {
			res  model.Result
			hint dnsserver.TransportHint
			err  error
		}{out, hint, err}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for up.Packets.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if up.Packets.Load() == 0 {
		t.Fatal("upstream never saw the query")
	}
	chaos.EmergencyDisable(store, eng)
	close(hold)
	select {
	case o := <-done:
		if o.err != nil {
			t.Fatal(o.err)
		}
		if o.hint != dnsserver.HintSend || o.res.RCode != model.RCodeNoError {
			t.Fatalf("want base answer, hint=%s rcode=%s", o.hint, o.res.RCode)
		}
		if len(o.res.Answers) != 1 || o.res.Answers[0].TTL == 0 {
			t.Fatalf("must not apply post-phase TTL/RCODE: %+v", o.res.Answers)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("query did not finish")
	}
}

func TestEmergencyDisableThroughListenerNoSERVFAIL(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{delayPolicy("slow", "a1", model.PhaseBeforeResponse, time.Hour, 0)}
	h, store, eng := buildChaos(t, st, nil)
	srv, err := dnsserver.New(dnsserver.Config{
		UDPAddr: "127.0.0.1:0", TCPAddr: "127.0.0.1:0", Handler: h,
		QueryTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() { _ = srv.Shutdown(t.Context()) })

	q := packQuery(t, "ns.lab.example.", model.TypeA, false)
	type read struct {
		raw []byte
	}
	ch := make(chan read, 1)
	go func() {
		ch <- read{raw: exchangeUDPTimeout(t, srv.UDPAddr(), q, 2*time.Second)}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for eng.Budgets().InFlight() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	chaos.EmergencyDisable(store, eng)
	got := <-ch
	if got.raw == nil {
		t.Fatal("expected a DNS answer after emergency, not silence")
	}
	msg, err := dnswire.UnpackUpstream(got.raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode == model.RCodeServFail {
		t.Fatal("emergency must not inject SERVFAIL")
	}
	if msg.RCode != model.RCodeNoError || len(msg.Answers) == 0 {
		t.Fatalf("want base answer, got rcode=%s answers=%v", msg.RCode, msg.Answers)
	}
}

func TestDelayPhasesBeforeAndAfterUpstream(t *testing.T) {
	up := startQueryFake(t)
	up.setAnswers(model.RR{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.9"})
	st := chaosState(t)
	st.Spec.Forwarding = model.ForwardingSpec{
		Policies: []model.ForwardingPolicy{{ID: "def", Suffix: ".", UpstreamPool: "p"}},
		Pools: []model.UpstreamPool{{
			ID: "p", Strategy: model.StrategyOrdered,
			Upstreams: []model.Upstream{{ID: "u1", Endpoint: up.UDPAddr(), Transport: model.TransportUDP}},
		}},
	}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "d1", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{Owners: []model.Name{"out.example."}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "d", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionDelay, Phase: model.PhaseBeforeUpstream, Duration: 40 * time.Millisecond},
		}}},
	}}
	h, _, _ := buildChaos(t, st, nil)
	q := model.Query{Name: "out.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP, RD: true}
	start := time.Now()
	res := serve(t, h, q)
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond {
		t.Fatalf("before-upstream delay %s", elapsed)
	}

	st.Spec.Chaos.Policies[0].Outcomes[0].Actions[0].Phase = model.PhaseAfterUpstream
	h, _, _ = buildChaos(t, st, nil)
	start = time.Now()
	res = serve(t, h, q)
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond {
		t.Fatalf("after-upstream delay %s", elapsed)
	}
}

func TestDelayAboveQueryTimeoutStillAnswers(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{delayPolicy("slow", "a1", model.PhaseBeforeResponse, 250*time.Millisecond, 0)}
	h, _, _ := buildChaos(t, st, nil)
	srv, err := dnsserver.New(dnsserver.Config{
		UDPAddr: "127.0.0.1:0", TCPAddr: "127.0.0.1:0", Handler: h,
		QueryTimeout: 80 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() { _ = srv.Shutdown(t.Context()) })

	start := time.Now()
	out := exchangeUDPTimeout(t, srv.UDPAddr(), packQuery(t, "ns.lab.example.", model.TypeA, false), 2*time.Second)
	if out == nil {
		t.Fatal("delay above QueryTimeout must still answer")
	}
	msg, err := dnswire.UnpackUpstream(out)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode != model.RCodeNoError || len(msg.Answers) == 0 {
		t.Fatalf("rcode=%s answers=%v", msg.RCode, msg.Answers)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("expected full delay, elapsed %s", elapsed)
	}

	st = chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{delayPolicy("short", "a1", model.PhaseBeforeResponse, 40*time.Millisecond, 0)}
	h, _, _ = buildChaos(t, st, nil)
	srv, err = dnsserver.New(dnsserver.Config{
		UDPAddr: "127.0.0.1:0", TCPAddr: "127.0.0.1:0", Handler: h,
		QueryTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() { _ = srv.Shutdown(t.Context()) })
	out = exchangeUDPTimeout(t, srv.UDPAddr(), packQuery(t, "ns.lab.example.", model.TypeA, false), time.Second)
	if out == nil {
		t.Fatal("delay below QueryTimeout must answer")
	}
}

func TestRandomPolicyPhasesAgreeOnLivePath(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "rand", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"a1"}},
		Selector: model.ChaosSelector{Mode: model.SelectorRandom, Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "both", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionTTL, Value: chaos.TTLValueSet, TTL: 3 * time.Second, Phase: model.PhaseBeforeResponse},
			{Type: model.ActionRCode, Value: "SERVFAIL", Phase: model.PhaseBeforeResponse},
		}}},
	}}
	h, _, _ := buildChaos(t, st, nil)
	for i := 0; i < 8; i++ {
		res := serve(t, h, model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP})
		if res.RCode != model.RCodeServFail {
			t.Fatalf("iter %d rcode=%s", i, res.RCode)
		}
	}
}

func TestTTLAndAlternatePackets(t *testing.T) {
	st := chaosState(t)
	st.Spec.Chaos.Safety.AllowedAddressCIDRs = []string{"10.0.0.0/8"}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "alt", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"a1"}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "a", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionAlternate, Values: []string{"10.9.9.9"}},
			{Type: model.ActionTTL, Value: chaos.TTLValueSet, TTL: 7 * time.Second},
		}}},
	}}
	_, srv := startChaosServer(t, st)
	out := exchangeUDP(t, srv.UDPAddr(), packQuery(t, "ns.lab.example.", model.TypeA, false))
	msg, err := dnswire.UnpackUpstream(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Answers) != 1 || msg.Answers[0].Data != "10.9.9.9" {
		t.Fatalf("answers %+v", msg.Answers)
	}
	if msg.Answers[0].TTL != 7*time.Second {
		t.Fatalf("ttl=%s", msg.Answers[0].TTL)
	}
}

func chaosState(t *testing.T) *model.State {
	t.Helper()
	return &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "t"},
		Spec: model.Spec{
			Access: model.AccessSpec{
				UnknownClient: model.UnknownClientRefuseForward,
				ClientGroups: []model.ClientGroup{{
					ID: "lab", CIDRs: []string{"10.42.0.0/16"}, AllowForward: true,
				}},
			},
			Defaults: model.DefaultsSpec{TTL: 30 * time.Second, NegativeTTL: 10 * time.Second, CNAMEDepth: 8},
			Zones: []model.Zone{{
				ID: "z", Name: "lab.example.", Mode: model.ZoneModeAuthoritative,
				SOA: &model.SOA{Primary: "ns.lab.example.", Administrator: "h.lab.example.", Serial: "1", Refresh: time.Hour, Retry: time.Minute, Expire: time.Hour, Minimum: 10 * time.Second},
				Records: []model.Record{
					{ID: "a1", Owner: "ns", Type: model.TypeA, TTL: 30 * time.Second, Values: []string{"10.42.0.53"}, ChaosPolicyRefs: []model.PolicyID{}},
					{ID: "tools-wildcard-a", Owner: "*.tools", Type: model.TypeA, TTL: 30 * time.Second, Values: []string{"10.42.0.20"}},
				},
			}},
			Chaos: model.ChaosSpec{
				Enabled: true,
				Safety: model.SafetySpec{
					MaxDelay:             10 * time.Second,
					MaxConcurrentDelayed: 64,
					MaxDropProbability:   1,
				},
			},
		},
	}
}

func delayPolicy(id string, rec model.RecordID, phase string, d, _ time.Duration) model.ChaosPolicy {
	return model.ChaosPolicy{
		ID: model.PolicyID(id), Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{rec}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "s", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionDelay, Phase: phase, Distribution: model.DistFixed, Duration: d},
		}}},
	}
}

func uniformPolicy(id, rec string, min, max time.Duration) model.ChaosPolicy {
	return model.ChaosPolicy{
		ID: model.PolicyID(id), Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{model.RecordID(rec)}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "u", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Distribution: model.DistUniform, Min: min, Max: max},
		}}},
	}
}

func rcodePolicy(id, rc string, ede *model.EDE) model.ChaosPolicy {
	return model.ChaosPolicy{
		ID: model.PolicyID(id), Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"a1"}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "r", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionRCode, Value: rc, EDE: ede},
		}}},
	}
}

func transportPolicy(id, typ string, tr []model.Transport) model.ChaosPolicy {
	return model.ChaosPolicy{
		ID: model.PolicyID(id), Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"a1"}, Transports: tr},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "t", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{{Type: typ}}}},
	}
}

func handlerFromState(t *testing.T, st *model.State, c *cache.Cache) *Handler {
	t.Helper()
	h, _, _ := buildChaos(t, st, c)
	return h
}

func buildChaos(t *testing.T, st *model.State, c *cache.Cache) (*Handler, *snapshot.Store, *chaos.Engine) {
	t.Helper()
	if len(st.Spec.Zones) > 0 && len(st.Spec.Zones[0].Records) > 0 {
		for _, p := range st.Spec.Chaos.Policies {
			if len(p.Scope.RecordIDs) == 1 && p.Scope.RecordIDs[0] == "a1" {
				st.Spec.Zones[0].Records[0].ChaosPolicyRefs = appendUniqueRef(st.Spec.Zones[0].Records[0].ChaosPolicyRefs, p.ID)
			}
			if len(p.Scope.RecordIDs) == 1 && p.Scope.RecordIDs[0] == "tools-wildcard-a" {
				st.Spec.Zones[0].Records[1].ChaosPolicyRefs = appendUniqueRef(st.Spec.Zones[0].Records[1].ChaosPolicyRefs, p.ID)
			}
		}
	}
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	eng := chaos.NewEngine(nil, nil)
	h := NewOpts(Opts{Store: store, Engine: eng, Cache: c, Fwd: forwarder.NewRuntime(nil, nil, nil, nil)})
	return h, store, eng
}

func appendUniqueRef(ids []model.PolicyID, id model.PolicyID) []model.PolicyID {
	for _, e := range ids {
		if e == id {
			return ids
		}
	}
	return append(ids, id)
}

func chaosHandler(t *testing.T, p model.ChaosPolicy) (*Handler, *snapshot.Store) {
	t.Helper()
	st := chaosState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{p}
	h := handlerFromState(t, st, nil)
	return h, nil
}

func startChaosServer(t *testing.T, st *model.State) (*Handler, *dnsserver.Server) {
	t.Helper()
	h := handlerFromState(t, st, nil)
	srv, err := dnsserver.New(dnsserver.Config{
		UDPAddr: "127.0.0.1:0",
		TCPAddr: "127.0.0.1:0",
		Handler: h,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() { _ = srv.Shutdown(t.Context()) })
	return h, srv
}

func packQuery(t *testing.T, name string, typ model.RRType, edns bool) []byte {
	t.Helper()
	var e *dnswire.EDNS
	if edns {
		e = &dnswire.EDNS{UDPSize: 1232}
	}
	q, err := dnswire.PackQuery(7, model.Query{Name: model.Name(name), Type: typ, Class: model.ClassIN, RD: true}, e)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func exchangeUDPTimeout(t *testing.T, addr net.Addr, payload []byte, d time.Duration) []byte {
	t.Helper()
	c, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	testutil.MustClose(t, c)
	_ = c.SetDeadline(time.Now().Add(d))
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

func exchangeTCPMsg(t *testing.T, addr net.Addr, payload []byte) []byte {
	t.Helper()
	out, err := exchangeTCPMaybe(t, addr, payload, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func exchangeTCPMaybe(t *testing.T, addr net.Addr, payload []byte, d time.Duration) ([]byte, error) {
	t.Helper()
	c, err := net.Dial("tcp", addr.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(d))
	writeTCPFrame(t, c, payload)
	var hdr [2]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeTCPFrame(t *testing.T, c net.Conn, payload []byte) {
	t.Helper()
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	if _, err := c.Write(hdr[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
}

func hasTCFlag(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	return b[2]&0x02 != 0
}

func wireHasEDE(t *testing.T, raw []byte, code uint16, text string) bool {
	t.Helper()
	// RFC 8914 option 15: code(2) + length(2) + INFO-CODE(2) + EXTRA-TEXT.
	opt := make([]byte, 6+len(text))
	opt[0], opt[1] = 0, 15
	binary.BigEndian.PutUint16(opt[2:4], uint16(2+len(text)))
	binary.BigEndian.PutUint16(opt[4:6], code)
	copy(opt[6:], text)
	return containsBytes(raw, opt)
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		ok := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
