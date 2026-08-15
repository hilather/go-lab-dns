package dnsquery

import (
	"context"
	"sync/atomic"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/resolver"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Engine is reserved for chaos.Decide. This PR never calls it; pass nil.
type Engine interface{}

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
	denied atomic.Int64
}

// New returns a dnsserver.Handler. eng is unused until CHA-001.
func New(store *snapshot.Store, _ Engine, c *cache.Cache, log Logger, clk testutil.Clock) dnsserver.Handler {
	return NewOpts(Opts{Store: store, Cache: c, Log: log, Clock: clk})
}

// Opts is the test/production constructor surface.
type Opts struct {
	Store *snapshot.Store
	Cache *cache.Cache
	Log   Logger
	Clock testutil.Clock
	Rand  testutil.Rand
	Fwd   *forwarder.Runtime
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
	return &Handler{store: o.Store, cache: o.Cache, log: o.Log, clk: clk, fwd: fwd}
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
func (h *Handler) ServeDNS(ctx context.Context, req *model.Query) (*dnsserver.Response, dnsserver.TransportHint, error) {
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
	q := *req
	q.Name = canonicalName(string(q.Name))
	if q.Class == "" {
		q.Class = model.ClassIN
	}

	cl := classify(snap, q)
	res, err := h.answer(ctx, snap, q, cl)
	if err != nil {
		if ctx.Err() != nil {
			return nil, dnsserver.HintDrop, ctx.Err()
		}
		return dnsserver.NewResponse(model.Result{RCode: model.RCodeServFail}), dnsserver.HintSend, nil
	}
	applyRA(&res, cl)
	if res.Explanation != nil {
		res.Explanation.ClientGroupID = cl.Group
		res.Explanation.ForwardingID = cl.ForwardingID
		if res.Explanation.Revision == "" {
			res.Explanation.Revision = snap.Revision
		}
	}
	return dnsserver.NewResponse(res), dnsserver.HintSend, nil
}

func (h *Handler) answer(ctx context.Context, snap *snapshot.Snapshot, q model.Query, cl class) (model.Result, error) {
	if ent, ok := h.lookupCache(snap, q, cl); ok {
		return ent.Result, nil
	}

	var local model.Result
	haveLocal := false
	if cl.ZoneID != "" {
		res, err := resolver.Resolve(ctx, snap, q, cl.ZoneID)
		if err != nil {
			return model.Result{}, err
		}
		local = res
		haveLocal = true
		if !res.Fallthrough {
			h.storeCache(snap, q, cl, res, true)
			return res, nil
		}
	}

	if cl.ForwardingID == "" {
		h.denied.Add(1)
		h.logf("denied_forward group=%s zone=%s policy=%s", cl.Group, cl.ZoneID, cl.ForwardingID)
		return refused(snap, q, cl), nil
	}

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
				h.denied.Add(1)
				h.logf("denied_forward group=%s zone=%s policy=%s", cl.Group, cl.ZoneID, cl.ForwardingID)
				return refused(snap, q, cl), nil
			}
			exchangeID = tid
		}
	}
	up, err := h.fwd.Exchange(ctx, snap, fq, exchangeID)
	if err != nil {
		if ctx.Err() != nil {
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
	h.storeCache(snap, q, cl, up, false)
	return up, nil
}

func (h *Handler) lookupCache(snap *snapshot.Snapshot, q model.Query, cl class) (cache.Entry, bool) {
	if h.cache == nil {
		return cache.Entry{}, false
	}
	localKey := cache.Key{
		Revision: snap.Revision,
		Name:     q.Name,
		Type:     q.Type,
		Class:    q.Class,
		Local:    true,
	}
	if ent, ok := h.cache.Get(localKey, cache.GetOpts{}); ok {
		return ent, true
	}
	if cl.ForwardingID == "" {
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
	return h.cache.Get(upKey, cache.GetOpts{})
}

func (h *Handler) storeCache(snap *snapshot.Snapshot, q model.Query, cl class, res model.Result, local bool) {
	if h.cache == nil {
		return
	}
	if !cacheable(res) {
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
	}, cache.PutOpts{})
}

func cacheable(res model.Result) bool {
	switch res.RCode {
	case model.RCodeNoError, model.RCodeNXDomain:
		return !res.Fallthrough
	default:
		return false
	}
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
