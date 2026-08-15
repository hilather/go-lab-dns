package observability

// Warning codes are a bounded, stable Status DTO surface.
const (
	WarnNoSnapshot        = "no_active_snapshot"
	WarnStateDrifted      = "state_drifted"
	WarnUpstreamUnhealthy = "upstream_unhealthy"
	WarnChaosEmergency    = "chaos_emergency_disabled"
	WarnTelemetryDropped  = "telemetry_dropped"
	WarnListenerUnbound   = "listener_unbound"
	WarnCacheNearCapacity = "cache_near_capacity"
)

// MaxWarnings caps the Status warning list.
const MaxWarnings = 16

// Warning is one agent-readable operational note.
type Warning struct {
	Code    string
	Message string
}

// Facts are process observations used to evaluate health. Chaos fields
// are recorded as warnings only; they never flip live/ready/degraded.
type Facts struct {
	HasRuntimeRevision bool
	ProcessDown        bool
	DNSDown            bool
	MgmtDown           bool
	UnhealthyUpstreams int
	TelemetryDrops     int64
	Drifted            bool
	EmergencyChaos     bool
	ChaosEnabled       bool
	CacheEntries       int
	CacheMax           int
}

// Probe is liveness, readiness, and degraded plus bounded warnings.
type Probe struct {
	Live     bool
	Ready    bool
	Degraded bool
	Warnings []Warning
}

// Evaluate implements pack 09 health semantics:
//   - Live: process is serving (not ProcessDown).
//   - Ready: live, a runtime revision exists, required listeners are up.
//   - Degraded: ready, but some upstreams are unhealthy. Not unready.
//   - Chaos never affects Live, Ready, or Degraded.
func Evaluate(in Facts) Probe {
	p := Probe{
		Live:  !in.ProcessDown,
		Ready: !in.ProcessDown && in.HasRuntimeRevision && !in.DNSDown && !in.MgmtDown,
	}
	p.Degraded = p.Ready && in.UnhealthyUpstreams > 0

	add := func(code, msg string) {
		if len(p.Warnings) >= MaxWarnings {
			return
		}
		p.Warnings = append(p.Warnings, Warning{Code: code, Message: msg})
	}
	if !in.HasRuntimeRevision {
		add(WarnNoSnapshot, "no active compiled snapshot")
	}
	if in.DNSDown || in.MgmtDown {
		add(WarnListenerUnbound, "a required listener is not bound")
	}
	if in.Drifted {
		add(WarnStateDrifted, "runtime revision differs from bootstrap")
	}
	if in.UnhealthyUpstreams > 0 {
		add(WarnUpstreamUnhealthy, "one or more upstreams are unhealthy")
	}
	if in.TelemetryDrops > 0 {
		add(WarnTelemetryDropped, "telemetry samples were dropped under backpressure")
	}
	if in.CacheMax > 0 && in.CacheEntries*10 >= in.CacheMax*9 {
		add(WarnCacheNearCapacity, "process cache is at or above 90% of MaxEntries")
	}
	// Informational only — must not change Live/Ready/Degraded.
	if in.EmergencyChaos {
		add(WarnChaosEmergency, "chaos execution is emergency-disabled")
	}
	_ = in.ChaosEnabled
	return p
}
