package perf

import (
	"net/netip"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// LabClient is a source address inside the default lab client group.
var LabClient = netip.MustParseAddr("10.42.0.10")

// LabState is the shared PERF-001 zone: exact, wildcard, CNAME, negative,
// optional forwarding, cache, and two chaos policies (idle + delay).
//
// upstream is rewritten onto the default pool before compile when a fake
// upstream is in use. An empty upstream leaves a black-hole endpoint so
// local-only benches do not dial.
func LabState(upstream string) *model.State {
	if upstream == "" {
		upstream = "127.0.0.1:9"
	}
	return &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "perf-lab"},
		Spec: model.Spec{
			Access: model.AccessSpec{
				UnknownClient: model.UnknownClientRefuseForward,
				ClientGroups: []model.ClientGroup{{
					ID:           "lab",
					CIDRs:        []string{"10.42.0.0/16", "127.0.0.0/8"},
					AllowForward: true,
				}},
			},
			Defaults: model.DefaultsSpec{TTL: 30 * time.Second, NegativeTTL: 10 * time.Second, CNAMEDepth: 8},
			Zones: []model.Zone{{
				ID:   "lab-zone",
				Name: "lab.example.",
				Mode: model.ZoneModeAuthoritative,
				SOA: &model.SOA{
					Primary: "ns.lab.example.", Administrator: "hostmaster.lab.example.",
					Serial: "1", Refresh: time.Hour, Retry: time.Minute, Expire: 24 * time.Hour, Minimum: 10 * time.Second,
				},
				Nameservers: []model.Name{"ns.lab.example."},
				Records: []model.Record{
					{ID: "ns-a", Owner: "ns", Type: model.TypeA, TTL: 30 * time.Second, Values: []string{"10.42.0.53"}},
					{ID: "www-a", Owner: "www", Type: model.TypeA, TTL: 30 * time.Second, Values: []string{"10.42.0.80"}},
					{ID: "tools-wildcard-a", Owner: "*.tools", Type: model.TypeA, TTL: 30 * time.Second, Values: []string{"10.42.0.20"}},
					{ID: "grafana-cname", Owner: "grafana.tools", Type: model.TypeCNAME, TTL: 30 * time.Second, Values: []string{"gateway.lab.example."}},
					{ID: "gateway-a", Owner: "gateway", Type: model.TypeA, TTL: 30 * time.Second, Values: []string{"10.42.0.10"}},
					{ID: "ttl-a", Owner: "ttl", Type: model.TypeA, TTL: 7 * time.Second, Values: []string{"192.0.2.7"}},
				},
			}},
			Forwarding: model.ForwardingSpec{
				Policies: []model.ForwardingPolicy{{
					ID: "default", Suffix: ".", UpstreamPool: "default",
				}},
				Pools: []model.UpstreamPool{{
					ID: "default", Strategy: model.StrategyOrdered,
					Upstreams: []model.Upstream{{ID: "u1", Endpoint: upstream, Transport: model.TransportUDP}},
				}},
			},
			Cache: model.CacheSpec{
				Enabled:            true,
				MaxEntries:         SafeCacheMaxEntries,
				MinimumTTL:         time.Second,
				MaximumTTL:         5 * time.Minute,
				MaximumNegativeTTL: time.Minute,
			},
			Chaos: model.ChaosSpec{
				Enabled: true,
				Safety: model.SafetySpec{
					MaxDelay:             SafeMaxDelay,
					MaxConcurrentDelayed: SafeMaxConcurrentDelayed,
					MaxDropProbability:   1,
				},
				Policies: []model.ChaosPolicy{
					idlePolicy(),
					delayPolicy(40 * time.Millisecond),
				},
			},
		},
	}
}

func idlePolicy() model.ChaosPolicy {
	return model.ChaosPolicy{
		ID: "idle-www", Owner: "perf-lab", Reason: "measure configured-but-not-triggered overhead",
		Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"www-a"}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "idle", Probability: 0},
		Outcomes: []model.ChaosOutcome{{ID: "noop", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Distribution: model.DistFixed, Duration: time.Second},
		}}},
	}
}

func delayPolicy(d time.Duration) model.ChaosPolicy {
	return model.ChaosPolicy{
		ID: "slow-ns", Owner: "perf-lab", Reason: "bounded delay path",
		Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"ns-a"}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "slow", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "delayed", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Distribution: model.DistFixed, Duration: d},
		}}},
	}
}

// QueryExact is the local A hit used by benches.
func QueryExact() model.Query {
	return model.Query{Name: "www.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}

// QueryWildcard synthesizes from *.tools.
func QueryWildcard() model.Query {
	return model.Query{Name: "foo.tools.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}

// QueryNegative is an authoritative NXDOMAIN.
func QueryNegative() model.Query {
	return model.Query{Name: "no-such.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}

// QueryNODATA is an existing name with no AAAA.
func QueryNODATA() model.Query {
	return model.Query{Name: "www.lab.example.", Type: model.TypeAAAA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}

// QueryCNAME follows grafana.tools → gateway.
func QueryCNAME() model.Query {
	return model.Query{Name: "grafana.tools.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}

// QueryDelay hits the always-on delay policy.
func QueryDelay() model.Query {
	return model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}

// QueryIdle hits the probability-0 policy (configured, not triggered).
func QueryIdle() model.Query {
	return QueryExact()
}

// QueryForward is a name with no local zone (default suffix policy).
func QueryForward(name string) model.Query {
	return model.Query{Name: model.Name(name), Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
}
