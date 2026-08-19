package observability

import (
	"encoding/json"
	"sort"
)

// CatalogID is the versioned metrics/events document identifier.
// Rename or semantic change of a catalog metric requires a new ID or a
// documented deprecation window (docs/16).
const CatalogID = "labdns.dev/metrics/v1alpha1"

// CatalogRelPath is the generated catalog artifact.
const CatalogRelPath = "api/metrics/v1alpha1.json"

// Kind is a catalog metric type.
type Kind string

const (
	KindCounter   Kind = "counter"
	KindGauge     Kind = "gauge"
	KindHistogram Kind = "histogram"
)

// Metric names. These are an operational compatibility surface.
const (
	MetricDNSAdmitted           = "labdns_dns_admitted_total"
	MetricDNSQueries            = "labdns_dns_queries_total"
	MetricDNSQueryDuration      = "labdns_dns_query_duration_seconds"
	MetricDNSParse              = "labdns_dns_parse_total"
	MetricDNSAdmission          = "labdns_dns_admission_total"
	MetricDNSResponses          = "labdns_dns_responses_total"
	MetricDNSTCPEvents          = "labdns_dns_tcp_events_total"
	MetricDeniedForward         = "labdns_dns_denied_forward_total"
	MetricResolverOutcomes      = "labdns_resolver_outcomes_total"
	MetricCNAMEDepthFailures    = "labdns_resolver_cname_depth_failures_total"
	MetricCacheLookups          = "labdns_cache_lookups_total"
	MetricCacheEntries          = "labdns_cache_entries"
	MetricCacheEvictions        = "labdns_cache_evictions_total"
	MetricUpstreamExchanges     = "labdns_upstream_exchanges_total"
	MetricUpstreamDuration      = "labdns_upstream_exchange_duration_seconds"
	MetricUpstreamTimeouts      = "labdns_upstream_timeouts_total"
	MetricUpstreamTransportErr  = "labdns_upstream_transport_errors_total"
	MetricUpstreamHealth        = "labdns_upstream_health"
	MetricUpstreamFailovers     = "labdns_upstream_failovers_total"
	MetricChaosMatches          = "labdns_chaos_policy_matches_total"
	MetricChaosTriggers         = "labdns_chaos_policy_triggers_total"
	MetricChaosSkips            = "labdns_chaos_policy_skips_total"
	MetricChaosDelay            = "labdns_chaos_delay_seconds"
	MetricChaosDelayedRequests  = "labdns_chaos_delayed_requests"
	MetricChaosBudget           = "labdns_chaos_budget_saturations_total"
	MetricChaosEffects          = "labdns_chaos_effects_total"
	MetricChaosEmergency        = "labdns_chaos_emergency_disabled"
	MetricCapabilityCalls       = "labdns_capability_calls_total"
	MetricCapabilityDuration    = "labdns_capability_duration_seconds"
	MetricStateCompileDuration  = "labdns_state_compile_duration_seconds"
	MetricStateValidateDuration = "labdns_state_validation_duration_seconds"
	MetricRevisionConflicts     = "labdns_state_revision_conflicts_total"
	MetricIdempotencyHits       = "labdns_state_idempotency_hits_total"
	MetricAuthFailures          = "labdns_auth_failures_total"
	MetricStateGeneration       = "labdns_state_generation"
	MetricStateDrifted          = "labdns_state_drifted"
	MetricTelemetryDropped      = "labdns_telemetry_dropped_total"
)

// Stable event names. Do not log raw QNAME or client IP by default.
const (
	EventDNSQuery          = "dns.query"
	EventDNSParseFailed    = "dns.parse_failed"
	EventDNSAdmissionFail  = "dns.admission_failed"
	EventDeniedForward     = "dns.denied_forward"
	EventResolverOutcome   = "resolver.outcome"
	EventCacheLookup       = "cache.lookup"
	EventUpstreamExchange  = "upstream.exchange"
	EventUpstreamUnhealthy = "upstream.unhealthy"
	EventChaosDecide       = "chaos.decide"
	EventChaosEmergency    = "chaos.emergency"
	EventChaosBudget       = "chaos.budget"
	EventStateApply        = "state.apply"
	EventStateReset        = "state.reset"
	EventStateCompile      = "state.compile"
	EventCapabilityInvoke  = "capability.invoke"
	EventAuthFailure       = "auth.failure"
	EventTelemetryDrop     = "telemetry.drop"
	EventHealthChange      = "health.change"
	EventUISession         = "ui.session"
)

// AllowedLabels is the default bounded label set. Metric definitions may
// use only a subset. QNAME and client IP are never allowed.
var AllowedLabels = []string{
	"action",
	"capability",
	"client_group_class",
	"component",
	"event",
	"kind",
	"outcome",
	"phase",
	"policy_id",
	"qtype_class",
	"queue",
	"rcode",
	"reason",
	"result",
	"source",
	"transport",
	"upstream_id",
	"zone_id",
}

// ForbiddenLabels must never appear on a catalog metric or recorded sample.
var ForbiddenLabels = []string{
	"actor",
	"actor_id",
	"client",
	"client_ip",
	"detail",
	"err",
	"error",
	"error_text",
	"idempotency",
	"idempotency_key",
	"message",
	"name",
	"owner",
	"peer",
	"qname",
	"query",
	"query_name",
	"remote_addr",
	"source_ip",
	"src",
	"src_ip",
}

// MetricDef is one catalog row.
type MetricDef struct {
	Name   string   `json:"name"`
	Kind   Kind     `json:"kind"`
	Help   string   `json:"help"`
	Labels []string `json:"labels"`
	Unit   string   `json:"unit,omitempty"`
}

// EventDef is one stable structured-log event.
type EventDef struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// Document is the versioned catalog artifact.
type Document struct {
	ID              string      `json:"id"`
	Version         string      `json:"version"`
	AllowedLabels   []string    `json:"allowedLabels"`
	ForbiddenLabels []string    `json:"forbiddenLabels"`
	Metrics         []MetricDef `json:"metrics"`
	Events          []EventDef  `json:"events"`
}

// Metrics returns the frozen first-GA catalog in stable name order.
func Metrics() []MetricDef {
	defs := []MetricDef{
		{Name: MetricDNSAdmitted, Kind: KindCounter, Help: "Admitted DNS queries after parse.", Labels: []string{"transport"}},
		{Name: MetricDNSQueries, Kind: KindCounter, Help: "Completed DNS queries by bounded classes.", Labels: []string{"transport", "client_group_class", "qtype_class", "source", "rcode"}},
		{Name: MetricDNSQueryDuration, Kind: KindHistogram, Help: "DNS query handler latency.", Labels: []string{"transport", "source"}, Unit: "seconds"},
		{Name: MetricDNSParse, Kind: KindCounter, Help: "DNS parse outcomes.", Labels: []string{"result"}},
		{Name: MetricDNSAdmission, Kind: KindCounter, Help: "DNS admission decisions.", Labels: []string{"result", "rcode"}},
		{Name: MetricDNSResponses, Kind: KindCounter, Help: "Finished DNS responses including chaos transport actions.", Labels: []string{"transport", "rcode", "action"}},
		{Name: MetricDNSTCPEvents, Kind: KindCounter, Help: "TCP accept, reject, close, and reset events.", Labels: []string{"event"}},
		{Name: MetricDeniedForward, Kind: KindCounter, Help: "Queries that needed a forward and were refused.", Labels: []string{"result"}},
		{Name: MetricResolverOutcomes, Kind: KindCounter, Help: "Local resolver outcomes.", Labels: []string{"source", "zone_id"}},
		{Name: MetricCNAMEDepthFailures, Kind: KindCounter, Help: "CNAME chains that hit the depth cap.", Labels: []string{"zone_id"}},
		{Name: MetricCacheLookups, Kind: KindCounter, Help: "Process cache lookups.", Labels: []string{"result"}},
		{Name: MetricCacheEntries, Kind: KindGauge, Help: "Process cache occupancy.", Labels: []string{"kind"}},
		{Name: MetricCacheEvictions, Kind: KindCounter, Help: "Process cache evictions.", Labels: nil},
		{Name: MetricUpstreamExchanges, Kind: KindCounter, Help: "Upstream exchanges.", Labels: []string{"upstream_id", "rcode", "result"}},
		{Name: MetricUpstreamDuration, Kind: KindHistogram, Help: "Upstream exchange latency.", Labels: []string{"upstream_id"}, Unit: "seconds"},
		{Name: MetricUpstreamTimeouts, Kind: KindCounter, Help: "Upstream timeouts.", Labels: []string{"upstream_id"}},
		{Name: MetricUpstreamTransportErr, Kind: KindCounter, Help: "Upstream transport errors.", Labels: []string{"upstream_id"}},
		{Name: MetricUpstreamHealth, Kind: KindGauge, Help: "Query-driven upstream reachability (1=healthy).", Labels: []string{"upstream_id"}},
		{Name: MetricUpstreamFailovers, Kind: KindCounter, Help: "Upstream failovers.", Labels: []string{"upstream_id"}},
		{Name: MetricChaosMatches, Kind: KindCounter, Help: "Chaos policy evaluations.", Labels: []string{"policy_id", "result"}},
		{Name: MetricChaosTriggers, Kind: KindCounter, Help: "Triggered chaos outcomes.", Labels: []string{"policy_id", "outcome"}},
		{Name: MetricChaosSkips, Kind: KindCounter, Help: "Skipped chaos policies.", Labels: []string{"policy_id", "reason"}},
		{Name: MetricChaosDelay, Kind: KindHistogram, Help: "Applied chaos delay.", Labels: []string{"policy_id"}, Unit: "seconds"},
		{Name: MetricChaosDelayedRequests, Kind: KindGauge, Help: "In-flight delayed queries.", Labels: nil},
		{Name: MetricChaosBudget, Kind: KindCounter, Help: "Chaos budget saturations and clamps.", Labels: []string{"policy_id"}},
		{Name: MetricChaosEffects, Kind: KindCounter, Help: "Executed chaos effects.", Labels: []string{"policy_id", "action"}},
		{Name: MetricChaosEmergency, Kind: KindGauge, Help: "Emergency chaos inhibit (1=disabled).", Labels: nil},
		{Name: MetricCapabilityCalls, Kind: KindCounter, Help: "Control-plane capability invocations.", Labels: []string{"capability", "transport", "result"}},
		{Name: MetricCapabilityDuration, Kind: KindHistogram, Help: "Control-plane capability latency.", Labels: []string{"capability", "transport"}, Unit: "seconds"},
		{Name: MetricStateCompileDuration, Kind: KindHistogram, Help: "Snapshot compile duration.", Labels: nil, Unit: "seconds"},
		{Name: MetricStateValidateDuration, Kind: KindHistogram, Help: "State validation duration.", Labels: nil, Unit: "seconds"},
		{Name: MetricRevisionConflicts, Kind: KindCounter, Help: "Optimistic-concurrency revision conflicts.", Labels: nil},
		{Name: MetricIdempotencyHits, Kind: KindCounter, Help: "Idempotency-key cache hits.", Labels: nil},
		{Name: MetricAuthFailures, Kind: KindCounter, Help: "Management authentication and scope denials.", Labels: []string{"result"}},
		{Name: MetricStateGeneration, Kind: KindGauge, Help: "Active snapshot generation.", Labels: nil},
		{Name: MetricStateDrifted, Kind: KindGauge, Help: "Runtime drifted from bootstrap (1=yes).", Labels: nil},
		{Name: MetricTelemetryDropped, Kind: KindCounter, Help: "Telemetry samples dropped under backpressure or policy.", Labels: []string{"reason"}},
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	for i := range defs {
		defs[i].Labels = append([]string(nil), defs[i].Labels...)
		sort.Strings(defs[i].Labels)
	}
	return defs
}

// Events returns the frozen structured-log event catalog.
func Events() []EventDef {
	fields := []string{
		"timestamp", "level", "event", "component", "request_id", "trace_id",
		"state_revision", "generation", "transport", "capability", "result",
		"error_code", "zone_id", "policy_id", "upstream_id", "duration_ms",
		"actor_id",
	}
	names := []string{
		EventDNSQuery, EventDNSParseFailed, EventDNSAdmissionFail, EventDeniedForward,
		EventResolverOutcome, EventCacheLookup, EventUpstreamExchange, EventUpstreamUnhealthy,
		EventChaosDecide, EventChaosEmergency, EventChaosBudget,
		EventStateApply, EventStateReset, EventStateCompile,
		EventCapabilityInvoke, EventAuthFailure, EventTelemetryDrop, EventHealthChange,
		EventUISession,
	}
	out := make([]EventDef, len(names))
	for i, n := range names {
		out[i] = EventDef{Name: n, Fields: append([]string(nil), fields...)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LookupMetric returns the catalog definition for name.
func LookupMetric(name string) (MetricDef, bool) {
	def, ok := metricIndex[name]
	return def, ok
}

var metricIndex = func() map[string]MetricDef {
	defs := Metrics()
	m := make(map[string]MetricDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return m
}()

// Catalog returns the versioned document.
func Catalog() Document {
	return Document{
		ID:              CatalogID,
		Version:         "v1alpha1",
		AllowedLabels:   append([]string(nil), AllowedLabels...),
		ForbiddenLabels: append([]string(nil), ForbiddenLabels...),
		Metrics:         Metrics(),
		Events:          Events(),
	}
}

// RenderCatalog is the generated JSON artifact.
func RenderCatalog() ([]byte, error) {
	b, err := json.MarshalIndent(Catalog(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
