package model

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"
	"time"
)

func sampleState() State {
	expires := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	return State{
		APIVersion: APIVersionV1Alpha1,
		Kind:       KindLabDNS,
		Metadata: Metadata{
			Name:   "primary-lab",
			Labels: map[string]string{"env": "lab"},
		},
		Spec: Spec{
			Listeners: ListenersSpec{
				DNS:        DNSListenerSpec{Address: ":5353", Protocols: []Transport{TransportUDP, TransportTCP}},
				Management: MgmtListenerSpec{Address: ":8080", RESTPath: "/v1", MCPPath: "/mcp"},
			},
			Access: AccessSpec{
				UnknownClient: UnknownClientRefuseForward,
				ClientGroups: []ClientGroup{{
					ID:           "test-devices",
					CIDRs:        []string{"10.42.0.0/16"},
					ChaosExempt:  false,
					AllowForward: true,
				}},
			},
			Defaults: DefaultsSpec{
				TTL:         30 * time.Second,
				NegativeTTL: 10 * time.Second,
				CNAMEDepth:  DefaultCNAMEDepth,
			},
			Zones: []Zone{{
				ID:   "lab-zone",
				Name: "lab.example.net.",
				Mode: ZoneModeAuthoritative,
				SOA: &SOA{
					Primary:       "ns1.lab.example.net.",
					Administrator: "hostmaster.lab.example.net.",
					Serial:        "auto",
					Refresh:       time.Hour,
					Retry:         5 * time.Minute,
					Expire:        24 * time.Hour,
					Minimum:       10 * time.Second,
				},
				Nameservers: []Name{"ns1.lab.example.net."},
				Records: []Record{{
					ID:              "ns1-a",
					Owner:           "ns1",
					Type:            TypeA,
					TTL:             30 * time.Second,
					Values:          []string{"10.42.0.53"},
					ChaosPolicyRefs: []PolicyID{"slow-tools"},
				}},
			}},
			Forwarding: ForwardingSpec{
				Policies: []ForwardingPolicy{{
					ID:           "default-policy",
					Suffix:       ".",
					UpstreamPool: "default",
					Failover: FailoverSpec{
						Timeout:             time.Second,
						OnTimeout:           true,
						OnTransportError:    true,
						OnSERVFAIL:          true,
						OnREFUSED:           true,
						UDPTruncateRetryTCP: true,
					},
				}},
				Pools: []UpstreamPool{{
					ID:       "default",
					Strategy: StrategyHealthAware,
					Upstreams: []Upstream{{
						ID:        "default-1",
						Endpoint:  "10.0.0.54:53",
						Transport: TransportUDP,
					}},
				}},
			},
			Cache: CacheSpec{
				Enabled:            true,
				MaxEntries:         10000,
				MinimumTTL:         time.Second,
				MaximumTTL:         5 * time.Minute,
				MaximumNegativeTTL: time.Minute,
				StaleServing:       false,
			},
			Chaos: ChaosSpec{
				Enabled:           true,
				EmergencyDisabled: false,
				Safety: SafetySpec{
					ProtectedNames:        []Name{"dns.lab.example.net."},
					ProtectedClientGroups: []ClientGroupID{"management"},
					AllowedAddressCIDRs:   []string{"10.0.0.0/8"},
					MaxDelay:              10 * time.Second,
					MaxConcurrentDelayed:  2000,
					MaxDropProbability:    0.5,
				},
				Policies: []ChaosPolicy{{
					ID:          "slow-tools",
					Owner:       "platform-lab",
					Reason:      "Test application startup timeouts",
					Enabled:     false,
					ExpiresAt:   &expires,
					SafetyClass: SafetyClassLow,
					Scope: ChaosScope{
						RecordIDs:    []RecordID{"tools-wildcard-a"},
						ClientGroups: []ClientGroupID{"test-devices"},
					},
					Selector: ChaosSelector{
						Mode:        SelectorDeterministic,
						Seed:        "startup-v1",
						Probability: 1.0,
						TimeBucket:  time.Second,
					},
					Outcomes: []ChaosOutcome{{
						ID:     "delayed",
						Weight: 100,
						Actions: []ChaosAction{{
							Type:         ActionDelay,
							Phase:        PhaseBeforeResponse,
							Distribution: DistUniform,
							Min:          100 * time.Millisecond,
							Max:          750 * time.Millisecond,
						}},
					}},
					Composition: CompositionCompose,
				}},
			},
			Observability: ObservabilitySpec{LogQNAME: false},
			Management:    ManagementSpec{Auth: AuthSpec{Profile: AuthProfileDevLoopbackUnauth}},
		},
	}
}

func TestStateJSONRoundTrip(t *testing.T) {
	in := sampleState()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("keys: %v", err)
	}
	for _, k := range []string{"apiVersion", "kind", "metadata", "spec"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("missing JSON key %q in %s", k, raw)
		}
	}

	var out State
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch\n in=%#v\nout=%#v", in, out)
	}
}

func TestOperationJSONRoundTrip(t *testing.T) {
	in := Operation{
		Op:     OpAdd,
		Target: Target{Kind: TargetRecord, ID: "ns1-a", ZoneID: "lab-zone"},
		Value:  json.RawMessage(`{"id":"ns1-a","owner":"ns1","type":"A"}`),
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("keys: %v", err)
	}
	for _, k := range []string{"op", "target", "value"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("missing JSON key %q in %s", k, raw)
		}
	}

	var out Operation
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Op != out.Op || in.Target != out.Target {
		t.Fatalf("round-trip header mismatch in=%+v out=%+v", in, out)
	}
	var inVal, outVal any
	if err := json.Unmarshal(in.Value, &inVal); err != nil {
		t.Fatalf("in value: %v", err)
	}
	if err := json.Unmarshal(out.Value, &outVal); err != nil {
		t.Fatalf("out value: %v", err)
	}
	if !reflect.DeepEqual(inVal, outVal) {
		t.Fatalf("value mismatch in=%v out=%v", inVal, outVal)
	}
}

func TestOperationJSONEachTargetKind(t *testing.T) {
	for _, kind := range AllTargetKinds {
		op := Operation{
			Op:     OpUpdate,
			Target: Target{Kind: kind, ID: "x"},
			Value:  json.RawMessage(`{"ok":true}`),
		}
		raw, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("%s marshal: %v", kind, err)
		}
		var out Operation
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s unmarshal: %v", kind, err)
		}
		if out.Target.Kind != kind {
			t.Fatalf("kind = %q, want %q", out.Target.Kind, kind)
		}
	}
}

func TestQueryJSONRoundTrip(t *testing.T) {
	in := Query{
		Name:      "ns1.lab.example.net.",
		Type:      TypeA,
		Class:     ClassIN,
		Client:    netip.MustParseAddr("10.42.0.9"),
		Transport: TransportUDP,
		RD:        true,
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Query
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch in=%#v out=%#v", in, out)
	}
}

func TestZeroStateJSONRoundTrip(t *testing.T) {
	var in State
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out State
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("zero State changed: %#v", out)
	}
}
