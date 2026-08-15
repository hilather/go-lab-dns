package config

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestNormalizeCopyOnWrite(t *testing.T) {
	in := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
	}
	in.Spec.Defaults.TTL = 0
	got, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if in.Spec.Defaults.TTL != 0 {
		t.Fatalf("input mutated: ttl=%s", in.Spec.Defaults.TTL)
	}
	if in.Spec.Access.UnknownClient != "" {
		t.Fatal("input unknownClient mutated")
	}
	if got.Spec.Defaults.TTL != DefaultTTL {
		t.Fatalf("ttl=%s", got.Spec.Defaults.TTL)
	}
	if got.Spec.Access.UnknownClient != model.UnknownClientRefuseForward {
		t.Fatalf("unknownClient=%q", got.Spec.Access.UnknownClient)
	}
	if got == in {
		t.Fatal("Normalize returned the same pointer")
	}
}

func TestNormalizeMaterializesListenerAndCNAMEDefaults(t *testing.T) {
	in := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
	}
	got, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Listeners.DNS.Address != DefaultDNSAddress {
		t.Fatalf("dns=%q", got.Spec.Listeners.DNS.Address)
	}
	if got.Spec.Listeners.Management.Address != DefaultMgmtAddress {
		t.Fatalf("mgmt=%q", got.Spec.Listeners.Management.Address)
	}
	if got.Spec.Listeners.Management.RESTPath != DefaultRESTPath || got.Spec.Listeners.Management.MCPPath != DefaultMCPPath {
		t.Fatalf("paths rest=%q mcp=%q", got.Spec.Listeners.Management.RESTPath, got.Spec.Listeners.Management.MCPPath)
	}
	if got.Spec.Defaults.CNAMEDepth != model.DefaultCNAMEDepth {
		t.Fatalf("cnameDepth=%d", got.Spec.Defaults.CNAMEDepth)
	}
	if got.Spec.Defaults.NegativeTTL != DefaultNegativeTTL {
		t.Fatalf("neg=%s", got.Spec.Defaults.NegativeTTL)
	}
	if len(got.Spec.Listeners.DNS.Protocols) != 2 {
		t.Fatalf("protocols=%v", got.Spec.Listeners.DNS.Protocols)
	}
	if got.Spec.Access.ClientGroups == nil {
		t.Fatal("clientGroups is nil; want empty slice")
	}
	if got.Spec.Management.Auth.Profile != model.AuthProfileDevLoopbackUnauth {
		t.Fatalf("auth.profile=%q", got.Spec.Management.Auth.Profile)
	}
}

func TestNormalizeDoesNotFlipExplicitAllowForwardFalse(t *testing.T) {
	in := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
		Spec: model.Spec{
			Access: model.AccessSpec{
				ClientGroups: []model.ClientGroup{{
					ID:           "g",
					CIDRs:        []string{"10.0.0.0/8"},
					AllowForward: false,
				}},
			},
		},
	}
	got, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Access.ClientGroups[0].AllowForward {
		t.Fatal("Normalize must not treat explicit/zero AllowForward as true")
	}
}

func TestNormalizeIdempotentAndExpandsOwners(t *testing.T) {
	in := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
		Spec: model.Spec{
			Zones: []model.Zone{{
				ID:   "z",
				Name: "Lab.Example.NET",
				Mode: model.ZoneModeOverlay,
				Records: []model.Record{{
					ID:     "r",
					Owner:  "Ns1",
					Type:   model.TypeA,
					Values: []string{"10.0.0.1"},
				}},
			}},
		},
	}
	a, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Normalize(a)
	if err != nil {
		t.Fatal(err)
	}
	if a.Spec.Zones[0].Name != "lab.example.net." {
		t.Fatalf("zone name=%q", a.Spec.Zones[0].Name)
	}
	if a.Spec.Zones[0].Records[0].Owner != "ns1.lab.example.net." {
		t.Fatalf("owner=%q", a.Spec.Zones[0].Records[0].Owner)
	}
	if a.Spec.Zones[0].Records[0].TTL != DefaultTTL {
		t.Fatalf("record ttl=%s", a.Spec.Zones[0].Records[0].TTL)
	}
	ra, _ := Revision(a)
	rb, _ := Revision(b)
	if ra != rb {
		t.Fatalf("normalize not idempotent %s vs %s", ra, rb)
	}
}

func TestNormalizeRejectsNonASCII(t *testing.T) {
	in := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
		Spec: model.Spec{
			Zones: []model.Zone{{
				ID:   "z",
				Name: "café.example.",
				Mode: model.ZoneModeOverlay,
			}},
		},
	}
	_, err := Normalize(in)
	requireValidation(t, err, violationNonASCII)
}

func TestNormalizeExpandsMXSRVAndUnderscoreOwners(t *testing.T) {
	in := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "x"},
		Spec: model.Spec{
			Zones: []model.Zone{{
				ID:   "z",
				Name: "lab.example.net.",
				Mode: model.ZoneModeOverlay,
				Records: []model.Record{
					{ID: "mx", Owner: "lab", Type: "mx", Values: []string{"10 mail"}},
					{ID: "srv", Owner: "_sip._tcp", Type: "srv", Values: []string{"0 1 5060 sip"}},
					{ID: "acme", Owner: "_acme-challenge", Type: model.TypeTXT, Values: []string{"ok"}},
				},
			}},
		},
	}
	got, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	r := got.Spec.Zones[0].Records
	if r[0].Type != model.TypeMX || r[0].Values[0] != "10 mail.lab.example.net." {
		t.Fatalf("mx=%q %q", r[0].Type, r[0].Values[0])
	}
	if r[1].Owner != "_sip._tcp.lab.example.net." || r[1].Values[0] != "0 1 5060 sip.lab.example.net." {
		t.Fatalf("srv owner=%q value=%q", r[1].Owner, r[1].Values[0])
	}
	if r[2].Owner != "_acme-challenge.lab.example.net." {
		t.Fatalf("acme owner=%q", r[2].Owner)
	}
}

func TestZeroValueAllowForwardRemainsFalse(t *testing.T) {
	var g model.ClientGroup
	if g.AllowForward {
		t.Fatal("unmaterialized zero AllowForward is true")
	}
	if DefaultTTL != 30*time.Second || DefaultNegativeTTL != 10*time.Second {
		t.Fatalf("ttl defaults %s %s", DefaultTTL, DefaultNegativeTTL)
	}
}
