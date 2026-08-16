package app

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/resolver"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// GetState returns revision metadata and a copy of Canonical.
func (s *App) GetState(ctx context.Context, actor Actor) (*StateView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	copied, err := cloneState(snap.Canonical)
	if err != nil {
		return nil, err
	}
	return &StateView{
		BootstrapRevision: snap.BootstrapRevision,
		RuntimeRevision:   snap.Revision,
		Generation:        snap.Generation,
		Drifted:           drifted(snap),
		LoadedAt:          snap.CompiledAt,
		Canonical:         auth.RedactState(copied),
	}, nil
}

func (s *App) ListZones(ctx context.Context, actor Actor, page Page) (*ZoneList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	zones := make([]model.Zone, len(snap.Canonical.Spec.Zones))
	for i, z := range snap.Canonical.Spec.Zones {
		zones[i] = copyZone(z)
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	start, err := pageOffset(page.Cursor)
	if err != nil {
		return nil, err
	}
	lim := pageLimit(page)
	if start > len(zones) {
		start = len(zones)
	}
	end := start + lim
	if end > len(zones) {
		end = len(zones)
	}
	out := make([]model.Zone, end-start)
	copy(out, zones[start:end])
	next := ""
	if end < len(zones) {
		next = strconv.Itoa(end)
	}
	return &ZoneList{Zones: out, NextCursor: next}, nil
}

func (s *App) GetZone(ctx context.Context, actor Actor, id model.ZoneID) (*model.Zone, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	for _, z := range snap.Canonical.Spec.Zones {
		if z.ID == id {
			copied := copyZone(z)
			return &copied, nil
		}
	}
	return nil, domainerr.NotFound("zone " + string(id) + " not found")
}

func (s *App) ListRecords(ctx context.Context, actor Actor, zone model.ZoneID, page Page) (*RecordList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	z, err := s.GetZone(ctx, actor, zone)
	if err != nil {
		return nil, err
	}
	recs := make([]model.Record, len(z.Records))
	for i, r := range z.Records {
		recs[i] = copyRecord(r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID < recs[j].ID })
	start, err := pageOffset(page.Cursor)
	if err != nil {
		return nil, err
	}
	lim := pageLimit(page)
	if start > len(recs) {
		start = len(recs)
	}
	end := start + lim
	if end > len(recs) {
		end = len(recs)
	}
	out := make([]model.Record, end-start)
	copy(out, recs[start:end])
	next := ""
	if end < len(recs) {
		next = strconv.Itoa(end)
	}
	return &RecordList{Records: out, NextCursor: next}, nil
}

func (s *App) GetRecord(ctx context.Context, actor Actor, zone model.ZoneID, id model.RecordID) (*model.Record, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	z, err := s.GetZone(ctx, actor, zone)
	if err != nil {
		return nil, err
	}
	for _, r := range z.Records {
		if r.ID == id {
			copied := copyRecord(r)
			return &copied, nil
		}
	}
	return nil, domainerr.NotFound("record " + string(id) + " not found")
}

func (s *App) Resolve(ctx context.Context, actor Actor, in ResolveIn) (*ResolveOut, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	res, err := s.resolveAgainst(ctx, snap, in)
	if err != nil {
		return nil, err
	}
	return &ResolveOut{Result: res}, nil
}

func (s *App) Explain(ctx context.Context, actor Actor, in ResolveIn) (*ExplainOut, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	// Explain is a live walk; cache would hide the compiled path.
	in.UseCache = false
	in.ApplyChaos = false
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	res, err := s.resolveAgainst(ctx, snap, in)
	if err != nil {
		return nil, err
	}
	return &ExplainOut{Result: res, Explanation: res.Explanation}, nil
}

func (s *App) resolveAgainst(ctx context.Context, snap *snapshot.Snapshot, in ResolveIn) (model.Result, error) {
	q := model.Query{
		Name:      canonicalQueryName(in.Name),
		Type:      in.Type,
		Class:     in.Class,
		Client:    in.Client,
		Transport: in.Transport,
		RD:        in.RD,
		CD:        in.CD,
	}
	if q.Type == "" {
		q.Type = model.TypeA
	}
	if q.Class == "" {
		q.Class = model.ClassIN
	}
	if q.Transport == "" {
		q.Transport = model.TransportUDP
	}
	zoneID, _ := snap.Zones.Select(q.Name)
	if in.UseCache && s.cache != nil {
		key := cache.Key{
			Revision: snap.Revision,
			Name:     q.Name,
			Type:     q.Type,
			Class:    q.Class,
			CD:       q.CD,
			Local:    true,
		}
		if ent, ok := s.cache.Get(key, cache.GetOpts{}); ok {
			return ent.Result, nil
		}
		res, err := resolver.Resolve(ctx, snap, q, zoneID)
		if err != nil {
			return model.Result{}, err
		}
		s.annotateExplanation(&res, snap, in)
		s.cache.Put(key, cache.Entry{Result: res}, cache.PutOpts{})
		return res, nil
	}
	res, err := resolver.Resolve(ctx, snap, q, zoneID)
	if err != nil {
		return model.Result{}, err
	}
	s.annotateExplanation(&res, snap, in)
	return res, nil
}

func (s *App) annotateExplanation(res *model.Result, snap *snapshot.Snapshot, in ResolveIn) {
	if res.Explanation == nil {
		return
	}
	res.Explanation.Revision = snap.Revision
	if in.ClientGroup != "" {
		res.Explanation.ClientGroupID = in.ClientGroup
		return
	}
	if in.Client.IsValid() {
		id, _ := snap.Access.Classify(in.Client)
		res.Explanation.ClientGroupID = id
	}
}

func (s *App) ListForwardingPolicies(ctx context.Context, actor Actor) ([]model.ForwardingPolicy, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	out := make([]model.ForwardingPolicy, len(snap.Canonical.Spec.Forwarding.Policies))
	for i, p := range snap.Canonical.Spec.Forwarding.Policies {
		out[i] = p
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *App) ListUpstreamPools(ctx context.Context, actor Actor) ([]model.UpstreamPool, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	out := make([]model.UpstreamPool, len(snap.Canonical.Spec.Forwarding.Pools))
	for i, p := range snap.Canonical.Spec.Forwarding.Pools {
		out[i] = copyPool(p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *App) UpstreamsStatus(ctx context.Context, actor Actor) ([]UpstreamStatus, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	var out []UpstreamStatus
	for _, p := range snap.Canonical.Spec.Forwarding.Pools {
		for _, u := range p.Upstreams {
			st := UpstreamStatus{
				ID:        u.ID,
				PoolID:    p.ID,
				Endpoint:  u.Endpoint,
				Transport: u.Transport,
				Healthy:   true,
			}
			if s.health != nil {
				st.Healthy = s.health.Healthy(u.ID)
			}
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PoolID != out[j].PoolID {
			return out[i].PoolID < out[j].PoolID
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *App) CacheStatus(ctx context.Context, actor Actor) (*CacheSummary, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if s.cache == nil {
		return &CacheSummary{}, nil
	}
	pol := s.cache.Policy()
	st := s.cache.Stats()
	return &CacheSummary{
		Enabled:    s.cache.Enabled(),
		MaxEntries: pol.MaxEntries,
		Entries:    st.Entries,
		Hits:       st.Hits,
		Misses:     st.Misses,
		Evicts:     st.Evicts,
	}, nil
}

func (s *App) CacheFlush(ctx context.Context, actor Actor, in FlushIn) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	_ = actor
	_ = in
	if s.cache != nil {
		s.cache.Flush()
	}
	return nil
}

func pageOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0, domainerr.ValidationFailed("invalid cursor",
			domainerr.FieldViolation{Path: "cursor", Code: "invalid_value", Message: "cursor must be an offset produced by this service"})
	}
	return n, nil
}

func copyZone(z model.Zone) model.Zone {
	z.Nameservers = append([]model.Name(nil), z.Nameservers...)
	if z.SOA != nil {
		soa := *z.SOA
		z.SOA = &soa
	}
	recs := make([]model.Record, len(z.Records))
	for i, r := range z.Records {
		recs[i] = copyRecord(r)
	}
	z.Records = recs
	return z
}

func copyRecord(r model.Record) model.Record {
	r.Values = append([]string(nil), r.Values...)
	r.ChaosPolicyRefs = append([]model.PolicyID(nil), r.ChaosPolicyRefs...)
	if r.GenericRDATA != nil {
		g := *r.GenericRDATA
		r.GenericRDATA = &g
	}
	return r
}

func copyPool(p model.UpstreamPool) model.UpstreamPool {
	p.Upstreams = append([]model.Upstream(nil), p.Upstreams...)
	return p
}

func copyChaosPolicy(p model.ChaosPolicy) model.ChaosPolicy {
	if p.Labels != nil {
		labels := make(map[string]string, len(p.Labels))
		for k, v := range p.Labels {
			labels[k] = v
		}
		p.Labels = labels
	}
	p.StartsAt = cloneTime(p.StartsAt)
	p.ExpiresAt = cloneTime(p.ExpiresAt)
	p.Scope = copyChaosScope(p.Scope)
	if p.Budget != nil {
		b := *p.Budget
		p.Budget = &b
	}
	if p.Outcomes != nil {
		outs := make([]model.ChaosOutcome, len(p.Outcomes))
		for i, o := range p.Outcomes {
			outs[i] = o
			if o.Actions != nil {
				acts := make([]model.ChaosAction, len(o.Actions))
				for j, a := range o.Actions {
					acts[j] = a
					acts[j].Values = append([]string(nil), a.Values...)
					if a.EDE != nil {
						e := *a.EDE
						acts[j].EDE = &e
					}
				}
				outs[i].Actions = acts
			}
		}
		p.Outcomes = outs
	}
	return p
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func copyChaosScope(s model.ChaosScope) model.ChaosScope {
	s.RecordIDs = append([]model.RecordID(nil), s.RecordIDs...)
	s.Owners = append([]model.Name(nil), s.Owners...)
	s.WildcardSourceIDs = append([]model.RecordID(nil), s.WildcardSourceIDs...)
	s.Zones = append([]model.ZoneID(nil), s.Zones...)
	s.ForwardingIDs = append([]model.PolicyID(nil), s.ForwardingIDs...)
	s.UpstreamPools = append([]model.PoolID(nil), s.UpstreamPools...)
	s.ClientGroups = append([]model.ClientGroupID(nil), s.ClientGroups...)
	s.QTypes = append([]model.RRType(nil), s.QTypes...)
	s.Transports = append([]model.Transport(nil), s.Transports...)
	return s
}

func canonicalQueryName(n model.Name) model.Name {
	s := strings.ToLower(strings.TrimSpace(string(n)))
	if s == "" {
		return "."
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return model.Name(s)
}
