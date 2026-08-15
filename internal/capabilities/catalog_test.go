package capabilities

import (
	"testing"
)

func TestCatalogStructure(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatal(err)
	}
	all := All()
	if len(all) != TableRowCount {
		t.Fatalf("len(All())=%d want %d", len(all), TableRowCount)
	}
	if _, ok := Lookup(HealthLive); !ok {
		t.Fatal("Lookup(health.live) missing")
	}
	if _, ok := Lookup(ID("not-a-capability")); ok {
		t.Fatal("Lookup unknown succeeded")
	}
	live := MustLookup(HealthLive)
	if !live.RESTOnly || live.MCP != nil {
		t.Fatalf("health.live must be REST-only: %+v", live)
	}
	ready := MustLookup(HealthReady)
	if !ready.RESTOnly {
		t.Fatal("health.ready must be REST-only")
	}
}

func TestFrozenIDsStable(t *testing.T) {
	want := []ID{
		HealthLive, HealthReady, Version, CapabilitiesID, Status, SchemaConfig,
		StateGet, StateValidate, ChangePlan, ChangeApply, StateExport, StateReset,
		Zones, Records, Resolve, ResolveExplain, ForwardingPolicies, UpstreamPools,
		UpstreamsStatus, CacheStatus, CacheFlush, ChaosStatus, ChaosPolicies,
		ChaosSimulate, ChaosActivate, ChaosSetExpiry, ChaosEmergency,
		AuditList, AuditGet, DocsDNSSemantics, DocsChaosSafety,
	}
	got := All()
	if len(got) != len(want) {
		t.Fatalf("catalog ids=%d frozen=%d (row missing or added without updating frozen list)", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("row %d id=%q want %q (renamed?)", i, got[i].ID, want[i])
		}
	}
}

func TestLookupRESTAndTool(t *testing.T) {
	c, ok := LookupREST("GET", "/v1/state")
	if !ok || c.ID != StateGet {
		t.Fatalf("LookupREST GET /v1/state = %+v ok=%v", c, ok)
	}
	tools := LookupTool("dns_change_apply")
	if len(tools) != 1 || tools[0].ID != ChangeApply {
		t.Fatalf("LookupTool apply = %+v", tools)
	}
	docs := LookupTool("dns_docs_get")
	if len(docs) != 2 {
		t.Fatalf("dns_docs_get should bind both docs rows, got %d", len(docs))
	}
	if _, ok := LookupResource("labdns://status"); !ok {
		t.Fatal("missing labdns://status")
	}
	if _, ok := LookupREST("GET", "/v1/nope"); ok {
		t.Fatal("unexpected REST hit")
	}
}

func TestHealthHasNoTools(t *testing.T) {
	for _, id := range []ID{HealthLive, HealthReady} {
		c := MustLookup(id)
		if !c.RESTOnly {
			t.Errorf("%s RESTOnly=false", id)
		}
		if c.MCP != nil {
			t.Errorf("%s has MCP binding %+v", id, c.MCP)
		}
		if len(c.RequiredScopes) != 0 {
			t.Errorf("%s scopes=%v want none", id, c.RequiredScopes)
		}
	}
	for _, name := range Tools() {
		if name == "dns_health_live" || name == "dns_health_ready" || name == "health.live" {
			t.Fatalf("health leaked as tool %q", name)
		}
	}
}

func TestAllReturnsCopy(t *testing.T) {
	a := All()
	a[0].Title = "mutated"
	if All()[0].Title == "mutated" {
		t.Fatal("All() shares the catalog slice")
	}
	apply := MustLookup(ChangeApply)
	apply.MCP.Tools[0] = "renamed_tool"
	again := MustLookup(ChangeApply)
	if again.MCP.Tools[0] != "dns_change_apply" {
		t.Fatal("Lookup shares MCP tool slice")
	}
}

func TestMustLookupUnknownPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustLookup("nope")
}

func TestDiscoveryDedupesDocsTool(t *testing.T) {
	var docs int
	var health int
	for _, d := range DiscoveryList() {
		if d.Name == "dns_docs_get" {
			docs++
		}
		if d.Name == string(HealthLive) || d.Name == string(HealthReady) {
			health++
		}
	}
	if docs != 1 {
		t.Fatalf("dns_docs_get listed %d times", docs)
	}
	if health != 2 {
		t.Fatalf("health discovery names=%d", health)
	}
}
