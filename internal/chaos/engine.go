package chaos

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Engine evaluates compiled policies. Clock/Rand are injected. Budgets are
// process-scoped and are never touched by Simulate.
type Engine struct {
	clock    testutil.Clock
	rng      testutil.Rand
	budget   *Budgets
	pressure *Pressure
	metrics  *Metrics
}

// Metrics is process-local effect counters. Labels never include QNAME.
type Metrics struct {
	Delayed        atomic.Int64
	DelayCanceled  atomic.Int64
	BudgetSkipped  atomic.Int64
	Drops          atomic.Int64
	Truncations    atomic.Int64
	TCPCloses      atomic.Int64
	TCPResets      atomic.Int64
	Holds          atomic.Int64
	RCodes         atomic.Int64
	TTL            atomic.Int64
	Alternates     atomic.Int64
	CacheHooks     atomic.Int64
	UpstreamHooks  atomic.Int64
	PressureReject atomic.Int64
}

// NewEngine returns an engine. Nil clock/rng become the system sources.
func NewEngine(clk testutil.Clock, rng testutil.Rand) *Engine {
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	if rng == nil {
		rng = testutil.SystemRand{}
	}
	return &Engine{clock: clk, rng: rng, budget: NewBudgets(), pressure: NewPressure(), metrics: &Metrics{}}
}

// Clock is the injected time source used by delay execution.
func (e *Engine) Clock() testutil.Clock {
	if e == nil {
		return testutil.SystemClock{}
	}
	return e.clock
}

// Budgets returns the process reservation table (CHA-002 delay path).
func (e *Engine) Budgets() *Budgets {
	if e == nil {
		return nil
	}
	return e.budget
}

// PressureTracker returns the process-scoped QPS/concurrency table.
func (e *Engine) PressureTracker() *Pressure {
	if e == nil {
		return nil
	}
	return e.pressure
}

// Stats returns process-local effect counters.
func (e *Engine) Stats() *Metrics {
	if e == nil {
		return nil
	}
	return e.metrics
}

// CancelDelays cancels outstanding reserved delays (emergency disable).
func (e *Engine) CancelDelays() {
	if e != nil && e.budget != nil {
		e.budget.CancelAll()
	}
}

// Decide selects outcomes for a classified query. It does not sleep, send
// packets, or write cache. Delay reservations are taken only for live
// (non-simulation) calls that include a delay action.
func (e *Engine) Decide(ctx context.Context, snap *snapshot.Snapshot, in DecisionIn) (ActionPlan, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return ActionPlan{}, err
		}
	}
	return e.decide(snap, in, nil, false)
}

// Simulate is Decide without budget consumption, sleeps, or mutation.
func (e *Engine) Simulate(ctx context.Context, snap *snapshot.Snapshot, in SimulateIn) (SimulateOut, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return SimulateOut{}, err
		}
	}
	plan, err := e.decide(snap, DecisionIn{
		Query:           in.Query,
		ClientGroupID:   in.ClientGroupID,
		ZoneID:          in.ZoneID,
		ForwardingID:    in.ForwardingID,
		Base:            in.Base,
		Phase:           in.Phase,
		SimulationNonce: in.Nonce,
	}, in.PolicyIDs, true)
	if err != nil {
		return SimulateOut{}, err
	}
	triggered := false
	for _, d := range plan.Decisions {
		if d.Triggered {
			triggered = true
			break
		}
	}
	return SimulateOut{
		Algorithm:  AlgorithmID,
		Disabled:   plan.Disabled,
		Reason:     plan.Reason,
		Triggered:  triggered,
		Decisions:  plan.Decisions,
		Plan:       plan,
		BudgetUsed: false,
	}, nil
}

func (e *Engine) decide(snap *snapshot.Snapshot, in DecisionIn, filter []model.PolicyID, simulate bool) (ActionPlan, error) {
	plan := ActionPlan{Algorithm: AlgorithmID}
	if e == nil {
		plan.Disabled = true
		plan.Reason = "no_engine"
		return plan, nil
	}
	if snap == nil {
		plan.Disabled = true
		plan.Reason = "no_snapshot"
		return plan, nil
	}
	if snap.EmergencyChaosOff || (snap.Canonical != nil && snap.Canonical.Spec.Chaos.EmergencyDisabled) {
		plan.Disabled = true
		plan.Reason = "emergency_disabled"
		return plan, nil
	}
	enabled := snap.Chaos.Enabled
	if !snap.Chaos.Compiled() && snap.Canonical != nil {
		enabled = snap.Canonical.Spec.Chaos.Enabled
	}
	if !enabled {
		plan.Disabled = true
		plan.Reason = "chaos_disabled"
		return plan, nil
	}

	if reason := protectedReason(snap, in); reason != "" {
		plan.Disabled = true
		plan.Reason = reason
		return plan, nil
	}

	now := e.clock.Now()
	recs, wildcard := contributingRecords(snap, in)
	pool := poolOf(snap, in.ForwardingID)
	cands := snap.Chaos.Candidates(snapshot.ChaosMatch{
		RecordIDs:      recs,
		WildcardSource: wildcard,
		Owner:          in.Query.Name,
		ZoneID:         in.ZoneID,
		ForwardingID:   in.ForwardingID,
		PoolID:         pool,
		ClientGroup:    in.ClientGroupID,
	})

	exclusive := map[string]model.PolicyID{}
	haveTransport := ""
	safety := snap.Safety

	for _, cp := range cands {
		if cp == nil {
			continue
		}
		p := cp.Policy
		if !containsPolicyFilter(filter, p.ID) {
			continue
		}
		if !scopeMatches(p, in, recs, wildcard, pool) {
			plan.Decisions = append(plan.Decisions, PolicyDecision{
				PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: "scope_mismatch",
			})
			continue
		}
		if !policyHasPhase(p, in.Phase) {
			continue
		}
		if p.Composition == model.CompositionExclusiveGroup && p.ExclusiveGroup != "" {
			if winner, ok := exclusive[p.ExclusiveGroup]; ok {
				plan.Decisions = append(plan.Decisions, PolicyDecision{
					PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: "exclusive_group:" + string(winner),
				})
				continue
			}
		}
		if reason := gateSkip(p, now, safety, snap.CompiledAt); reason != "" {
			// Simulation answers "what would this policy do if activated".
			// Live Decide still skips disabled policies.
			if !(simulate && reason == "disabled") {
				plan.Decisions = append(plan.Decisions, PolicyDecision{
					PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: reason,
				})
				continue
			}
		}

		h, pval, wval := e.uniforms(p, snap, in, now, simulate)
		if reason := everyNthSkip(now, p.Selector, h.U0); reason != "" {
			plan.Decisions = append(plan.Decisions, PolicyDecision{
				PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: reason, Hash: h,
			})
			continue
		}
		prob := p.Selector.Probability
		if canDrop(p) && safety.MaxDropProbability > 0 && prob > safety.MaxDropProbability {
			prob = safety.MaxDropProbability
		}
		if pval >= prob {
			plan.Decisions = append(plan.Decisions, PolicyDecision{
				PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: "probability", Hash: h,
			})
			continue
		}
		out, ok := PickOutcome(p.Outcomes, wval)
		if !ok {
			plan.Decisions = append(plan.Decisions, PolicyDecision{
				PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: "no_outcome", Hash: h,
			})
			continue
		}

		acts, clamps, delay, early, hint, skipRes, transportConflict := e.planActions(p, out, in, snap, h, now, simulate)
		if transportConflict && haveTransport != "" {
			plan.Decisions = append(plan.Decisions, PolicyDecision{
				PolicyID: p.ID, Precedence: cp.Precedence, SkipReason: "transport_conflict", Hash: h, OutcomeID: out.ID,
			})
			continue
		}
		dec := PolicyDecision{
			PolicyID:   p.ID,
			Precedence: cp.Precedence,
			OutcomeID:  out.ID,
			Triggered:  true,
			Hash:       h,
			Actions:    acts,
		}
		plan.Decisions = append(plan.Decisions, dec)
		plan.Actions = append(plan.Actions, acts...)
		plan.Clamped = append(plan.Clamped, clamps...)
		if delay > plan.Delay {
			plan.Delay = delay
		}
		if early != "" && plan.EarlyRCode == "" {
			plan.EarlyRCode = early
			plan.SkipResolve = skipRes
		}
		if hint != "" {
			haveTransport = hint
			plan.TransportHint = hint
		}
		if p.Composition == model.CompositionExclusiveGroup && p.ExclusiveGroup != "" {
			exclusive[p.ExclusiveGroup] = p.ID
		}
		if p.Composition == model.CompositionTerminal {
			break
		}
	}
	fillPlanSummary(&plan)
	return plan, nil
}

func (e *Engine) uniforms(p model.ChaosPolicy, snap *snapshot.Snapshot, in DecisionIn, now time.Time, simulate bool) (HashResult, float64, float64) {
	mode := p.Selector.Mode
	if mode == "" {
		mode = model.SelectorDeterministic
	}
	if mode == model.SelectorRandom && !simulate {
		h := HashResult{}
		if in.Sticky != nil {
			h = in.Sticky.Draw(p.ID, e.rng)
		} else {
			h = randomDraw(e.rng)
		}
		return h, h.P, h.W
	}
	// Deterministic, and simulation of random: hash-v1.
	// Field 10 is empty on the live path; only Simulate fills it.
	rev := p.Selector.Revision
	if rev == "" {
		rev = snap.Revision
	}
	client := ""
	if in.Query.Client.IsValid() {
		client = in.Query.Client.String()
	}
	nonce := ""
	if simulate {
		nonce = in.SimulationNonce
	}
	h := HashV1(HashFields{
		Seed:        p.Selector.Seed,
		Revision:    rev,
		PolicyID:    p.ID,
		QNAME:       canonicalName(in.Query.Name),
		QTYPE:       in.Query.Type,
		ClientGroup: ClientGroupField(p.Selector, in.ClientGroupID, client),
		Transport:   in.Query.Transport,
		TimeBucket:  TimeBucketString(now, p.Selector.TimeBucket),
		Nonce:       nonce,
	})
	return h, h.P, h.W
}

func (e *Engine) planActions(p model.ChaosPolicy, out model.ChaosOutcome, in DecisionIn, snap *snapshot.Snapshot, h HashResult, now time.Time, simulate bool) (acts []PlannedAction, clamps []ClampRecord, delay time.Duration, early model.RCode, hint string, skipRes bool, transportConflict bool) {
	globalMax := snap.Safety.MaxDelay
	policyMax := time.Duration(0)
	if p.Budget != nil {
		policyMax = p.Budget.MaxDelay
	}
	for _, a := range out.Actions {
		phase := actionPhaseOf(a)
		if !actionInPhase(phase, in.Phase) && in.Phase != "" {
			continue
		}
		pa := PlannedAction{
			PolicyID:     p.ID,
			OutcomeID:    out.ID,
			Type:         a.Type,
			Phase:        phase,
			Distribution: a.Distribution,
			Value:        a.Value,
			TTL:          a.TTL,
			Min:          a.Min,
			Max:          a.Max,
			Limit:        a.Limit,
			UpstreamID:   a.UpstreamID,
			Hold:         a.Hold,
			Seed:         h.U1,
			Values:       append([]string(nil), a.Values...),
		}
		if a.EDE != nil {
			e := *a.EDE
			pa.EDE = &e
		}
		switch a.Type {
		case model.ActionDelay:
			clamped, u1, recs := e.mapDelay(p, a, in, snap, h, now, simulate, globalMax, policyMax)
			pa.Delay = clamped
			pa.Seed = u1
			if len(recs) > 0 {
				pa.Clamped = true
				clamps = append(clamps, recs...)
			}
			if clamped > delay {
				delay = clamped
			}
		case model.ActionRCode:
			pa.RCode = a.Value
			if in.Phase == PhasePreResolution || in.Phase == "" {
				early = injectedRCode(a.Value)
				skipRes = true
			}
		case model.ActionDrop:
			if hint != "" && hint != "drop" {
				transportConflict = true
				continue
			}
			hint = "drop"
			pa.Value = "drop"
		case model.ActionTruncate:
			if hint != "" && hint != "truncate" {
				transportConflict = true
				continue
			}
			hint = "truncate"
		case model.ActionTCPClose:
			if hint != "" && hint != "tcp-close" {
				transportConflict = true
				continue
			}
			hint = "tcp-close"
		case model.ActionTCPReset:
			if hint != "" && hint != "tcp-reset" {
				transportConflict = true
				continue
			}
			hint = "tcp-reset"
		case model.ActionUpstream:
			if normalizeActionValue(a.Value) == UpstreamValueDelay {
				clamped, u1, recs := e.mapDelay(p, a, in, snap, h, now, simulate, globalMax, policyMax)
				pa.Delay = clamped
				pa.Seed = u1
				if len(recs) > 0 {
					pa.Clamped = true
					clamps = append(clamps, recs...)
				}
			}
		case model.ActionPressure:
			if p.Budget != nil {
				pa.Rate = p.Budget.MaxRate
				pa.Concurrency = p.Budget.MaxConcurrency
			}
		}
		acts = append(acts, pa)
	}
	return acts, clamps, delay, early, hint, skipRes, transportConflict
}

func (e *Engine) mapDelay(p model.ChaosPolicy, a model.ChaosAction, in DecisionIn, snap *snapshot.Snapshot, h HashResult, now time.Time, simulate bool, globalMax, policyMax time.Duration) (time.Duration, uint64, []ClampRecord) {
	d := a.Duration
	if d == 0 {
		d = a.TTL
	}
	u1 := h.U1
	if a.Distribution == model.DistUniform || (a.Distribution == "" && (a.Min != 0 || a.Max != 0)) {
		if p.Selector.Mode != model.SelectorRandom || simulate {
			nonce := ""
			if simulate {
				nonce = in.SimulationNonce
			}
			delayFields := HashFields{
				Seed:        p.Selector.Seed,
				Revision:    p.Selector.Revision,
				PolicyID:    p.ID,
				QNAME:       canonicalName(in.Query.Name),
				QTYPE:       in.Query.Type,
				ClientGroup: ClientGroupField(p.Selector, in.ClientGroupID, clientString(in)),
				Transport:   in.Query.Transport,
				TimeBucket:  TimeBucketString(now, p.Selector.TimeBucket),
				Nonce:       DelayNonce(nonce),
			}
			if delayFields.Revision == "" {
				delayFields.Revision = snap.Revision
			}
			u1 = HashV1(delayFields).U1
		} else if h.U1 != 0 {
			u1 = h.U1
		} else if e != nil {
			u1 = e.rng.Uint64()
		}
		d = UniformDelay(a.Min, a.Max, u1)
	}
	clamped, did := clampDelay(d, globalMax, policyMax)
	var recs []ClampRecord
	if did {
		recs = []ClampRecord{{
			PolicyID: p.ID, Action: a.Type, Reason: "max_delay",
			From: d.String(), To: clamped.String(),
		}}
	}
	return clamped, u1, recs
}

func fillPlanSummary(plan *ActionPlan) {
	if plan == nil {
		return
	}
	for _, a := range plan.Actions {
		switch a.Type {
		case model.ActionCache:
			switch normalizeActionValue(a.Value) {
			case CacheValueBypass:
				plan.Cache.Bypass = true
			case CacheValueForceMiss, "miss":
				plan.Cache.ForceMiss = true
			case CacheValueExpire:
				plan.Cache.Expire = true
			case CacheValueStale:
				plan.Cache.ServeStale = true
			}
		case model.ActionUpstream:
			switch normalizeActionValue(a.Value) {
			case UpstreamValueDelay:
				if a.Delay > plan.Upstream.Delay {
					plan.Upstream.Delay = a.Delay
				}
			case UpstreamValueUnavailable:
				if a.UpstreamID != "" {
					plan.Upstream.Unavailable = append(plan.Upstream.Unavailable, a.UpstreamID)
				}
			case UpstreamValueForce, "select":
				if a.UpstreamID != "" {
					plan.Upstream.Force = a.UpstreamID
				}
			case UpstreamValueTimeout:
				plan.Upstream.Timeout = true
			case UpstreamValueTransportError:
				plan.Upstream.TransportError = true
			case UpstreamValueFailover:
				plan.Upstream.Failover = true
			default:
				if rc := injectedRCode(a.Value); rc != "" && a.Value != "" && !isCacheOrTTLValue(a.Value) {
					if isRCodeValue(a.Value) {
						plan.Upstream.SyntheticRCode = rc
					}
				}
			}
		case model.ActionPressure:
			if plan.Pressure.PolicyID == "" {
				plan.Pressure.PolicyID = a.PolicyID
				plan.Pressure.MaxRate = a.Rate
				plan.Pressure.MaxConc = a.Concurrency
				plan.Pressure.OnExceed = a.Value
				if plan.Pressure.OnExceed == "" {
					plan.Pressure.OnExceed = PressureValueServFail
				}
			}
		case model.ActionRCode:
			if a.EDE != nil && plan.EDE == nil {
				e := *a.EDE
				plan.EDE = &e
			}
		}
		if a.Hold > plan.Hold {
			plan.Hold = a.Hold
		}
		if a.Type == model.ActionDelay && a.Phase == model.PhaseBeforeUpstream && a.Delay > plan.Upstream.Delay {
			plan.Upstream.Delay = a.Delay
		}
	}
}

func injectedRCode(v string) model.RCode {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "NODATA":
		return model.RCodeNoError
	case string(model.RCodeServFail):
		return model.RCodeServFail
	case string(model.RCodeRefused):
		return model.RCodeRefused
	case string(model.RCodeNXDomain):
		return model.RCodeNXDomain
	case string(model.RCodeNoError):
		return model.RCodeNoError
	case string(model.RCodeFormErr):
		return model.RCodeFormErr
	case string(model.RCodeNotImp):
		return model.RCodeNotImp
	default:
		return model.RCode(strings.ToUpper(strings.TrimSpace(v)))
	}
}

func isRCodeValue(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "NODATA", "NOERROR", "SERVFAIL", "REFUSED", "NXDOMAIN", "FORMERR", "NOTIMP":
		return true
	default:
		return false
	}
}

func isCacheOrTTLValue(v string) bool {
	switch normalizeActionValue(v) {
	case CacheValueBypass, CacheValueForceMiss, "miss", CacheValueExpire, CacheValueStale,
		TTLValueSet, TTLValueClamp, TTLValueZero, TTLValueJitter,
		UpstreamValueDelay, UpstreamValueUnavailable, UpstreamValueForce, "select",
		UpstreamValueTimeout, UpstreamValueTransportError, UpstreamValueFailover:
		return true
	default:
		return false
	}
}

func normalizeActionValue(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func unit(u uint64) float64 { return float64(u) / two64 }

func canDrop(p model.ChaosPolicy) bool {
	for _, o := range p.Outcomes {
		for _, a := range o.Actions {
			if a.Type == model.ActionDrop {
				return true
			}
		}
	}
	return false
}

func policyHasPhase(p model.ChaosPolicy, want Phase) bool {
	if want == "" {
		return true
	}
	for _, o := range p.Outcomes {
		for _, a := range o.Actions {
			if actionInPhase(actionPhaseOf(a), want) {
				return true
			}
		}
		if len(o.Actions) == 0 {
			return true
		}
	}
	return false
}

func protectedReason(snap *snapshot.Snapshot, in DecisionIn) string {
	qname := canonicalName(in.Query.Name)
	for _, n := range snap.Safety.ProtectedNames {
		if snapshot.InZone(qname, n) {
			return "protected_name"
		}
	}
	if snap.Canonical != nil {
		for _, n := range snap.Canonical.Spec.Chaos.Safety.ProtectedNames {
			if snapshot.InZone(qname, n) {
				return "protected_name"
			}
		}
	}
	if in.ClientGroupID != "" {
		for _, g := range snap.Safety.ProtectedClientGroups {
			if g == in.ClientGroupID {
				return "protected_client_group"
			}
		}
		if snap.Canonical != nil {
			for _, g := range snap.Canonical.Spec.Access.ClientGroups {
				if g.ID == in.ClientGroupID && g.ChaosExempt {
					return "chaos_exempt"
				}
			}
		}
	}
	return ""
}

func contributingRecords(snap *snapshot.Snapshot, in DecisionIn) ([]model.RecordID, model.RecordID) {
	var recs []model.RecordID
	var wild model.RecordID
	if in.Base != nil && in.Base.WildcardSource != nil {
		wild = *in.Base.WildcardSource
		recs = append(recs, wild)
	}
	if snap == nil {
		return recs, wild
	}
	zd, ok := snap.Zones.Lookup(in.ZoneID)
	if !ok || zd == nil {
		return recs, wild
	}
	seen := map[model.RecordID]struct{}{}
	add := func(id model.RecordID) {
		if id == "" {
			return
		}
		if _, dup := seen[id]; dup {
			return
		}
		seen[id] = struct{}{}
		recs = append(recs, id)
	}
	for _, id := range recs {
		seen[id] = struct{}{}
	}
	if in.Base != nil {
		for _, rr := range in.Base.Answers {
			if set, ok := zd.RRset(rr.Name, rr.Type); ok {
				add(set.ID)
			}
		}
		return recs, wild
	}
	// Pre-resolution / simulate without a base: look up the already-selected
	// zone's exact or wildcard source. This is not zone rediscovery.
	qname := canonicalName(in.Query.Name)
	if set, ok := zd.RRset(qname, in.Query.Type); ok {
		add(set.ID)
	}
	if !zd.HasName(qname) {
		enc := zd.ClosestEncloser(qname)
		if enc != "" {
			if set, ok := zd.Wildcard(model.Name("*."+string(enc)), in.Query.Type); ok {
				wild = set.ID
				add(set.ID)
			}
		}
	}
	return recs, wild
}

func poolOf(snap *snapshot.Snapshot, fwd model.PolicyID) model.PoolID {
	if snap == nil || fwd == "" {
		return ""
	}
	p, ok := snap.Forwarding.Lookup(fwd)
	if !ok || p == nil {
		return ""
	}
	return p.PoolID
}

func canonicalName(n model.Name) model.Name {
	s := strings.ToLower(strings.TrimSpace(string(n)))
	if s == "" {
		return "."
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return model.Name(s)
}

func clientString(in DecisionIn) string {
	if in.Query.Client.IsValid() {
		return in.Query.Client.String()
	}
	return ""
}

// FormatU64 is a stable decimal for goldens.
func FormatU64(u uint64) string { return strconv.FormatUint(u, 10) }
