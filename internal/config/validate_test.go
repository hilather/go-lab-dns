package config

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestValidatePackSample(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "pack-sample.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(st); err != nil {
		t.Fatal(err)
	}
	if st.Spec.Access.UnknownClient != model.UnknownClientRefuseForward {
		t.Fatalf("unknownClient=%q", st.Spec.Access.UnknownClient)
	}
}

func TestValidateEmptyClientGroups(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "empty-client-groups.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Spec.Access.ClientGroups) != 0 {
		t.Fatalf("groups=%d", len(st.Spec.Access.ClientGroups))
	}
	if len(st.Spec.Forwarding.Policies) != 0 {
		t.Fatal("empty-client-groups should not forward")
	}
}

func TestValidateTimeBucket(t *testing.T) {
	st := minimalState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID:          "p",
		Owner:       "lab",
		Reason:      "bucket",
		SafetyClass: model.SafetyClassLow,
		Selector:    model.ChaosSelector{Mode: model.SelectorDeterministic, Probability: 1, TimeBucket: 500 * time.Millisecond},
		Outcomes:    []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Duration: time.Millisecond}}}},
	}}
	_ = requireValidation(t, Validate(st), violationTimeBucket)

	st.Spec.Chaos.Policies[0].Selector.TimeBucket = time.Second
	if err := Validate(st); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCNAMELoopMixedCaseType(t *testing.T) {
	doc := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: loop
spec:
  zones:
    - id: z
      name: lab.example.net.
      mode: overlay
      records:
        - id: a
          owner: a
          type: cname
          values: [b]
        - id: b
          owner: b
          type: CNAME
          values: [a]
`
	_, err := Load([]byte(doc))
	_ = requireValidation(t, err, violationCNAMELoop)
}

func TestValidateRejectsUppercaseTransport(t *testing.T) {
	st := minimalState(t)
	st.Spec.Forwarding.Pools = []model.UpstreamPool{{
		ID:       "p",
		Strategy: model.StrategyOrdered,
		Upstreams: []model.Upstream{{
			ID:        "u",
			Endpoint:  "10.0.0.1:53",
			Transport: "UDP",
		}},
	}}
	_ = requireValidation(t, Validate(st), violationInvalidTransport)
}

func TestValidateUnknownClientClosed(t *testing.T) {
	st := minimalState(t)
	st.Spec.Access.UnknownClient = "allow-forward"
	_ = requireValidation(t, Validate(st), violationInvalidValue)
}

func TestValidateTransportClosedNoDoT(t *testing.T) {
	st := minimalState(t)
	st.Spec.Forwarding.Pools = []model.UpstreamPool{{
		ID:       "p",
		Strategy: model.StrategyOrdered,
		Upstreams: []model.Upstream{{
			ID:        "u",
			Endpoint:  "10.0.0.1:853",
			Transport: "dot",
		}},
	}}
	_ = requireValidation(t, Validate(st), violationInvalidTransport)
}

func TestValidateDoesNotMutate(t *testing.T) {
	st := minimalState(t)
	before := st.Spec.Defaults.CNAMEDepth
	if err := Validate(st); err != nil {
		t.Fatal(err)
	}
	if st.Spec.Defaults.CNAMEDepth != before {
		t.Fatal("Validate mutated state")
	}
}

func TestValidateConflictingTransportActions(t *testing.T) {
	st := minimalState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID:          "p",
		Owner:       "lab",
		Reason:      "conflict",
		SafetyClass: model.SafetyClassLow,
		Selector:    model.ChaosSelector{Mode: model.SelectorDeterministic, Probability: 1},
		Outcomes: []model.ChaosOutcome{{
			ID:     "o",
			Weight: 1,
			Actions: []model.ChaosAction{
				{Type: model.ActionDrop},
				{Type: model.ActionTCPReset},
			},
		}},
	}}
	_ = requireValidation(t, Validate(st), violationConflict)
}

func TestValidateHighImpactRequiresExpiry(t *testing.T) {
	st := minimalState(t)
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID:          "p",
		Owner:       "lab",
		Reason:      "high",
		Enabled:     true,
		SafetyClass: model.SafetyClassHigh,
		Selector:    model.ChaosSelector{Mode: model.SelectorDeterministic, Probability: 1},
		Outcomes:    []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Duration: time.Millisecond}}}},
	}}
	_ = requireValidation(t, Validate(st), violationMissingExpiry)
}

func TestValidateAlternateAddressCIDR(t *testing.T) {
	st := minimalState(t)
	st.Spec.Chaos.Safety.AllowedAddressCIDRs = []string{"10.0.0.0/8"}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID:          "p",
		Owner:       "lab",
		Reason:      "alt",
		SafetyClass: model.SafetyClassLow,
		Selector:    model.ChaosSelector{Mode: model.SelectorDeterministic, Probability: 1},
		Outcomes: []model.ChaosOutcome{{
			ID:      "o",
			Weight:  1,
			Actions: []model.ChaosAction{{Type: model.ActionAlternate, Values: []string{"8.8.8.8"}}},
		}},
	}}
	_ = requireValidation(t, Validate(st), violationAltAddr)
}

func TestValidateDoesNotRequireClientGroups(t *testing.T) {
	st := minimalState(t)
	st.Spec.Access.ClientGroups = nil
	n, err := Normalize(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(n); err != nil {
		t.Fatal(err)
	}
}

func minimalState(t *testing.T) *model.State {
	t.Helper()
	st := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
		Spec: model.Spec{
			Zones: []model.Zone{{
				ID:   "z",
				Name: "lab.example.net.",
				Mode: model.ZoneModeOverlay,
				Records: []model.Record{{
					ID:     "r",
					Owner:  "ns1.lab.example.net.",
					Type:   model.TypeA,
					TTL:    30 * time.Second,
					Values: []string{"10.0.0.1"},
				}},
			}},
		},
	}
	n, err := Normalize(st)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
