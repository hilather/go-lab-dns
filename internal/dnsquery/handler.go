package dnsquery

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/chaos/effects"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/observability"
	"github.com/hilather/go-lab-dns/internal/resolver"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Engine is chaos.Decide. Nil skips chaos.
type Engine interface {
	Decide(ctx context.Context, snap *snapshot.Snapshot, in chaos.DecisionIn) (chaos.ActionPlan, error)
}

// Logger is an optional diagnostic sink. Nil is silent.
type Logger interface {
	Printf(format string, args ...any)
}

// Handler is the DNS orchestrator. One snapshot load per query.
type Handler struct {
	store  *snapshot.Store
	cache  *cache.Cache
	log    Logger
	clk    testutil.Clock
	fwd    *forwarder.Runtime
	eng    Engine
	obs    *observability.Registry
	denied atomic.Int64
}

// New returns a dnsserver.Handler.
func New(store *snapshot.Store, eng Engine, c *cache.Cache, log Logger, clk testutil.Clock) dnsserver.Handler {
	return NewOpts(Opts{Store: store, Engine: eng, Cache: c, Log: log, Clock: clk})
}

// Opts is the test/production constructor surface.
type Opts struct {
	Store   *snapshot.Store
	Engine  Engine
	Cache   *cache.Cache
	Log     Logger
	Clock   testutil.Clock
	Rand    testutil.Rand
	Fwd     *forwarder.Runtime
	Metrics *observability.Registry
}

// NewOpts builds a Handler with injected runtime (fake upstreams, clock).
func NewOpts(o Opts) *Handler {
	clk := o.Clock
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	fwd := o.Fwd
	if fwd == nil {
		fwd = forwarder.NewRuntime(clk, o.Rand, nil, nil)
	}
	return &Handler{store: o.Store, cache: o.Cache, log: o.Log, clk: clk, fwd: fwd, eng: o.Engine, obs: o.Metrics}
}

func (h *Handler) logf(format string, args ...any) {
	if h != nil && h.log != nil {
		h.log.Printf(format, args...)
	}
}

// DeniedForward is the count of queries that needed a forward and were
// refused (unknown/local-only, or no matching policy).
func (h *Handler) DeniedForward() int64 {
	if h == nil {
		return 0
	}
	return h.denied.Load()
}

// ServeDNS implements dnsserver.Handler.
func (h *Handler) ServeDNS(ctx context.Context, req *model.Query) (resp *dnsserver.Response, hint dnsserver.TransportHint, err error) {
	started := mono(h)
	var q model.Query
	var cl class
	var tracked bool
	defer func() {
		if !tracked {
			return
		}
		res := model.Result{}
		if resp != nil {
			res = resp.Result()
		}
		h.observeQuery(q, cl, res, started)
	}()
	if err := ctx.Err(); err != nil {
		return nil, dnsserver.HintDrop, err
	}
	if h == nil || h.store == nil {
		return dnsserver.NewResponse(model.Result{RCode: model.RCodeServFail}), dnsserver.HintSend, nil
	}
	snap := h.store.Load()
	if snap == nil {
		return dnsserver.NewResponse(model.Result{RCode: model.RCodeServFail}), dnsserver.HintSend, nil
	}
	if req == nil {
		return dnsserver.NewResponse(model.Result{RCode: model.RCodeFormErr}), dnsserver.HintSend, nil
	}
	q = *req
	q.Name = canonicalName(string(q.Name))
	if q.Class == "" {
		q.Class = model.ClassIN
	}

	cl = classify(snap, q)
	tracked = true
	sticky := chaos.NewStickyRand()
	exclusive := chaos.NewExclusiveSet()
	var pre chaos.ActionPlan
	if !h.inhibited() {
		pre = h.decide(ctx, snap, q, cl, nil, chaos.PhasePreResolution, sticky, exclusive)
	} else {
		pre = chaos.ActionPlan{Disabled: true, Reason: "emergency_disabled"}
	}
	sess := effects.NewSession(h.clk, h.budgets(), snap, h.metrics())
	defer sess.Release()

	if err := sess.Sleep(ctx, pre, model.PhaseBeforeResolution); err != nil {
		return nil, dnsserver.HintDrop, err
	}
	if h.inhibited() {
		return h.sendBase(ctx, snap, q, cl, model.Result{}, chaos.ActionPlan{})
	}

	press := effects.CheckPressure(h.pressure(), pre, h.clk.Now(), h.metrics())
	defer press.Release()
	if press.Exceeded {
		if press.Drop {
			return nil, dnsserver.HintDrop, nil
		}
		res := model.Result{RCode: press.RCode, Source: model.SourceNegative}
		applyRA(&res, cl)
		return dnsserver.NewResponse(res), dnsserver.HintSend, nil
	}

	var res model.Result
	if pre.SkipResolve && pre.EarlyRCode != "" {
		res = effects.EarlyFailure(pre, q)
	} else {
		var err error
		res, err = h.answer(ctx, snap, q, cl, pre, func() error {
			return sess.Sleep(ctx, pre, model.PhaseBeforeUpstream)
		})
		if err != nil {
			if dropOnCtx(ctx) {
				return nil, dnsserver.HintDrop, err
			}
			return dnsserver.NewResponse(model.Result{RCode: model.RCodeServFail}), dnsserver.HintSend, nil
		}
	}
	if h.inhibited() {
		applyRA(&res, cl)
		return dnsserver.NewResponse(res), dnsserver.HintSend, nil
	}

	post := h.decide(ctx, snap, q, cl, &res, chaos.PhaseResponse, sticky, exclusive)
	if h.inhibited() {
		applyRA(&res, cl)
		return dnsserver.NewResponse(res), dnsserver.HintSend, nil
	}
	base := res
	res = effects.ApplyResponse(res, post, q, h.metrics())
	effects.Annotate(&res, base, pre, post)
	applyRA(&res, cl)
	applyRA(&base, cl)
	if res.Explanation != nil {
		res.Explanation.ClientGroupID = cl.Group
		res.Explanation.ForwardingID = cl.ForwardingID
		if res.Explanation.Revision == "" {
			res.Explanation.Revision = snap.Revision
		}
	}
	if err := sess.Sleep(ctx, post, model.PhaseAfterUpstream); err != nil {
		return nil, dnsserver.HintDrop, err
	}
	if err := sess.Sleep(ctx, post, model.PhaseBeforeResponse); err != nil {
		return nil, dnsserver.HintDrop, err
	}
	if h.inhibited() {
		// Emergency after a response-phase mutate: send the pre-chaos answer.
		return dnsserver.NewResponse(base), dnsserver.HintSend, nil
	}

	hintPlan := post
	if hintPlan.TransportHint == "" {
		hintPlan = pre
	}
	hint = effects.Hint(hintPlan, q.Transport, h.metrics())
	resp = dnsserver.NewResponse(res)
	hold := hintPlan.Hold
	if hold == 0 {
		hold = pre.Hold
	}
	if hold > 0 {
		_ = resp.SetHoldFor(hold)
	}
	return resp, hint, nil
}

func (h *Handler) sendBase(ctx context.Context, snap *snapshot.Snapshot, q model.Query, cl class, have model.Result, plan chaos.ActionPlan) (*dnsserver.Response, dnsserver.TransportHint, error) {
	res := have
	if res.RCode == "" && len(res.Answers) == 0 {
		var err error
		res, err = h.answer(ctx, snap, q, cl, plan, nil)
		if err != nil {
			if dropOnCtx(ctx) {
				return nil, dnsserver.HintDrop, err
			}
			return dnsserver.NewResponse(model.Result{RCode: model.RCodeServFail}), dnsserver.HintSend, nil
		}
	}
	applyRA(&res, cl)
	return dnsserver.NewResponse(res), dnsserver.HintSend, nil
}

func (h *Handler) decide(ctx context.Context, snap *snapshot.Snapshot, q model.Query, cl class, base *model.Result, phase chaos.Phase, sticky *chaos.StickyRand, exclusive *chaos.ExclusiveSet) chaos.ActionPlan {
	if h == nil || h.eng == nil || h.inhibited() {
		return chaos.ActionPlan{Disabled: h.inhibited(), Reason: inhibitReason(h)}
	}
	plan, err := h.eng.Decide(ctx, snap, chaos.DecisionIn{
		Query:         q,
		ClientGroupID: cl.Group,
		ZoneID:        cl.ZoneID,
		ForwardingID:  cl.ForwardingID,
		Base:          base,
		Phase:         phase,
		Sticky:        sticky,
		Exclusive:     exclusive,
	})
	if err != nil {
		h.logf("chaos decide: %v", err)
		return chaos.ActionPlan{}
	}
	h.observeChaos(plan)
	return plan
}

func (h *Handler) inhibited() bool {
	return h != nil && h.store != nil && h.store.EmergencyChaosOff()
}

func inhibitReason(h *Handler) string {
	if h != nil && h.inhibited() {
		return "emergency_disabled"
	}
	return ""
}

func dropOnCtx(ctx context.Context) bool {
	if ctx == nil || ctx.Err() == nil {
		return false
	}
	return !errors.Is(ctx.Err(), context.DeadlineExceeded)
}

// liveWorkCtx keeps resolve/exchange runnable after a chaos delay consumed
// the listener's query deadline. Shutdown cancel still aborts.
func liveWorkCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx, func() {}
	}
	parent := dnsserver.ServerContext(ctx)
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, dnsserver.DefaultQueryTimeout)
}

func (h *Handler) answer(ctx context.Context, snap *snapshot.Snapshot, q model.Query, cl class, plan chaos.ActionPlan, beforeUpstream func() error) (model.Result, error) {
	if ent, ok := h.lookupCache(snap, q, cl, plan); ok {
		return ent.Result, nil
	}

	rctx, rcancel := liveWorkCtx(ctx)
	defer rcancel()

	var local model.Result
	haveLocal := false
	if cl.ZoneID != "" {
		res, err := resolver.Resolve(rctx, snap, q, cl.ZoneID)
		if err != nil {
			return model.Result{}, err
		}
		local = res
		haveLocal = true
		if !res.Fallthrough {
			h.storeCache(snap, q, cl, res, true, plan)
			return res, nil
		}
	}

	// Classification selects a policy from the original QNAME. Overlay CNAME
	// fallthrough must still re-select on the target (docs/02-dns-semantics.md),
	// even when the original name matches no suffix. Unknown and local-only
	// clients stay local: AllowForward is false and ForwardingID is empty.
	if !cl.AllowForward {
		return h.localOverlayOrRefused(snap, q, cl, local, haveLocal, plan), nil
	}
	if beforeUpstream != nil {
		if err := beforeUpstream(); err != nil {
			return model.Result{}, err
		}
	}
	xctx, xcancel := liveWorkCtx(ctx)
	defer xcancel()

	fq := q
	var prefix []model.RR
	exchangeID := cl.ForwardingID
	if haveLocal {
		if target := lastCNAMETarget(local.Answers); target != "" {
			fq.Name = target
			prefix = local.Answers
			// Ask the pool that matches the name sent upstream. cl.ForwardingID
			// stays the original-QNAME selection for a future chaos.Decide.
			tid, ok := snap.Forwarding.Select(target)
			if !ok {
				return h.localOverlayOrRefused(snap, q, cl, local, haveLocal, plan), nil
			}
			exchangeID = tid
		}
	}
	if exchangeID == "" {
		return h.localOverlayOrRefused(snap, q, cl, local, haveLocal, plan), nil
	}
	up, err := h.fwd.ExchangeOpts(xctx, snap, fq, exchangeID, effects.Exchange(plan, h.metrics()))
	if err != nil {
		if dropOnCtx(ctx) || dropOnCtx(xctx) {
			return model.Result{}, err
		}
		// Exchange itself returns SERVFAIL results rather than dial errors
		// for exhausted pools; a remaining error is fail-closed.
		up = model.Result{RCode: model.RCodeServFail, Source: model.SourceUpstream, ForwardingID: cl.ForwardingID}
	}
	if len(prefix) > 0 {
		up.Answers = append(append([]model.RR(nil), prefix...), up.Answers...)
	}
	if haveLocal {
		up.ZoneID = local.ZoneID
		up.ZoneMode = local.ZoneMode
	}
	h.storeCache(snap, q, cl, up, false, plan)
	return up, nil
}

func (h *Handler) lookupCache(snap *snapshot.Snapshot, q model.Query, cl class, plan chaos.ActionPlan) (cache.Entry, bool) {
	if h.cache == nil {
		return cache.Entry{}, false
	}
	opts := effects.CacheGet(plan, h.metrics())
	localKey := cache.Key{
		Revision: snap.Revision,
		Name:     q.Name,
		Type:     q.Type,
		Class:    q.Class,
		Local:    true,
	}
	if ent, ok := h.cache.Get(localKey, opts); ok && cache.Cacheable(ent.Result) {
		h.observeCache("hit")
		return ent, true
	}
	if cl.ForwardingID == "" {
		h.observeCache("miss")
		return cache.Entry{}, false
	}
	upKey := cache.Key{
		Revision:     snap.Revision,
		Name:         q.Name,
		Type:         q.Type,
		Class:        q.Class,
		CD:           q.CD,
		ForwardingID: cl.ForwardingID,
		Local:        false,
	}
	ent, ok := h.cache.Get(upKey, opts)
	if ok && cache.Cacheable(ent.Result) {
		h.observeCache("hit")
		return ent, true
	}
	h.observeCache("miss")
	return cache.Entry{}, false
}

func (h *Handler) storeCache(snap *snapshot.Snapshot, q model.Query, cl class, res model.Result, local bool, plan chaos.ActionPlan) {
	if h.cache == nil {
		return
	}
	if !cache.Cacheable(res) {
		return
	}
	if !local && cl.ForwardingID == "" {
		return
	}
	key := cache.Key{
		Revision: snap.Revision,
		Name:     q.Name,
		Type:     q.Type,
		Class:    q.Class,
		Local:    local,
	}
	if !local {
		key.CD = q.CD
		key.ForwardingID = cl.ForwardingID
	}
	h.cache.Put(key, cache.Entry{
		Result:   res,
		Negative: res.RCode == model.RCodeNXDomain || (res.RCode == model.RCodeNoError && len(res.Answers) == 0),
		Original: res.Source,
		Upstream: res.UpstreamID,
		Policy:   res.ForwardingID,
	}, effects.CachePut(plan))
}

func (h *Handler) liveEngine() *chaos.Engine {
	if h == nil {
		return nil
	}
	e, _ := h.eng.(*chaos.Engine)
	return e
}

func (h *Handler) budgets() *chaos.Budgets {
	if e := h.liveEngine(); e != nil {
		return e.Budgets()
	}
	return nil
}

func (h *Handler) metrics() *chaos.Metrics {
	if e := h.liveEngine(); e != nil {
		return e.Stats()
	}
	return nil
}

func (h *Handler) pressure() *chaos.Pressure {
	if e := h.liveEngine(); e != nil {
		return e.PressureTracker()
	}
	return nil
}

// localOverlayOrRefused keeps a local overlay CNAME (or other local
// answers) when forwarding is refused. REFUSED is only for no local path.
func (h *Handler) localOverlayOrRefused(snap *snapshot.Snapshot, q model.Query, cl class, local model.Result, haveLocal bool, plan chaos.ActionPlan) model.Result {
	if haveLocal && len(local.Answers) > 0 {
		local.Fallthrough = false
		h.storeCache(snap, q, cl, local, true, plan)
		return local
	}
	h.denied.Add(1)
	h.observeDenied(cl)
	h.logf("denied_forward group=%s zone=%s policy=%s", cl.Group, cl.ZoneID, cl.ForwardingID)
	return refused(snap, q, cl)
}

func applyRA(res *model.Result, cl class) {
	// RA is set only when forwarding is available to this client:
	// known group, AllowForward, and a selected policy.
	res.RA = cl.AllowForward && cl.ForwardingID != ""
}

func refused(snap *snapshot.Snapshot, q model.Query, cl class) model.Result {
	rev := model.Revision("")
	if snap != nil {
		rev = snap.Revision
	}
	return model.Result{
		RCode:        model.RCodeRefused,
		RA:           false,
		AA:           false,
		AD:           false,
		CD:           false,
		ForwardingID: "",
		Explanation: &model.Explanation{
			Query:         q,
			ClientGroupID: cl.Group,
			ZoneID:        cl.ZoneID,
			Revision:      rev,
		},
	}
}

func lastCNAMETarget(rrs []model.RR) model.Name {
	var last model.Name
	for _, rr := range rrs {
		if rr.Type == model.TypeCNAME && rr.Data != "" {
			last = canonicalName(rr.Data)
		}
	}
	return last
}

func (h *Handler) observeQuery(q model.Query, cl class, res model.Result, started time.Duration) {
	if h == nil || h.obs == nil {
		return
	}
	src := observability.SourceClass(string(res.Source))
	h.obs.Inc(observability.MetricDNSQueries, map[string]string{
		"transport":          string(q.Transport),
		"client_group_class": observability.ClientGroupClass(cl.Group != "", cl.AllowForward),
		"qtype_class":        observability.QTypeClass(string(q.Type)),
		"source":             src,
		"rcode":              string(res.RCode),
	}, 1)
	if res.ZoneID != "" {
		h.obs.Inc(observability.MetricResolverOutcomes, map[string]string{
			"source":  src,
			"zone_id": string(res.ZoneID),
		}, 1)
	}
	sec := 0.0
	if h.clk != nil {
		d := h.clk.Monotonic() - started
		if d > 0 {
			sec = d.Seconds()
		}
	}
	h.obs.Observe(observability.MetricDNSQueryDuration, map[string]string{
		"transport": string(q.Transport),
		"source":    src,
	}, sec)
}

func mono(h *Handler) time.Duration {
	if h != nil && h.clk != nil {
		return h.clk.Monotonic()
	}
	return 0
}

func (h *Handler) observeDenied(cl class) {
	if h == nil || h.obs == nil {
		return
	}
	result := "no_policy"
	if cl.Group == "" {
		result = "unknown"
	} else if !cl.AllowForward {
		result = "local_only"
	}
	h.obs.Inc(observability.MetricDeniedForward, map[string]string{"result": result}, 1)
}

func (h *Handler) observeCache(result string) {
	if h == nil || h.obs == nil {
		return
	}
	h.obs.Inc(observability.MetricCacheLookups, map[string]string{"result": result}, 1)
}

func (h *Handler) observeChaos(plan chaos.ActionPlan) {
	if h == nil || h.obs == nil {
		return
	}
	for _, d := range plan.Decisions {
		pid := string(d.PolicyID)
		if d.Triggered {
			h.obs.Inc(observability.MetricChaosMatches, map[string]string{"policy_id": pid, "result": "trigger"}, 1)
			if d.OutcomeID != "" {
				h.obs.Inc(observability.MetricChaosTriggers, map[string]string{"policy_id": pid, "outcome": d.OutcomeID}, 1)
			}
		} else {
			h.obs.Inc(observability.MetricChaosMatches, map[string]string{"policy_id": pid, "result": "skip"}, 1)
			if d.SkipReason != "" {
				h.obs.Inc(observability.MetricChaosSkips, map[string]string{"policy_id": pid, "reason": d.SkipReason}, 1)
			}
		}
		for _, a := range d.Actions {
			if a.Type == "" {
				continue
			}
			h.obs.Inc(observability.MetricChaosEffects, map[string]string{"policy_id": pid, "action": a.Type}, 1)
		}
	}
}
