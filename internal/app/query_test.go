package app

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestCopyOnWritePoolsPoliciesAndImpact(t *testing.T) {
	path := copyNamedFixture(t, "pack-sample.yaml")
	svc, boot := mustBoot(t, path)
	ctx := context.Background()

	pools, err := svc.ListUpstreamPools(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) == 0 || len(pools[0].Upstreams) == 0 {
		t.Fatal("pack-sample missing pools")
	}
	wantEP := pools[0].Upstreams[0].Endpoint
	poolID := pools[0].ID
	pools[0].Upstreams[0].Endpoint = "9.9.9.9:53"
	again, err := svc.ListUpstreamPools(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Upstreams[0].Endpoint != wantEP {
		t.Fatal("ListUpstreamPools leaked live upstreams")
	}
	for _, p := range svc.Store().Load().Canonical.Spec.Forwarding.Pools {
		if p.ID == poolID && p.Upstreams[0].Endpoint != wantEP {
			t.Fatal("caller mutated live pool")
		}
	}

	pol, err := svc.GetChaosPolicy(ctx, actor(), "slow-tools")
	if err != nil {
		t.Fatal(err)
	}
	if len(pol.Scope.RecordIDs) == 0 {
		t.Fatal("slow-tools missing record scope")
	}
	pol.Scope.RecordIDs[0] = "mutated"
	if pol.Labels == nil {
		pol.Labels = map[string]string{}
	}
	pol.Labels["x"] = "y"
	livePol, err := svc.GetChaosPolicy(ctx, actor(), "slow-tools")
	if err != nil {
		t.Fatal(err)
	}
	if livePol.Scope.RecordIDs[0] == "mutated" {
		t.Fatal("GetChaosPolicy leaked scope")
	}

	exp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	plan, err := svc.Plan(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetChaosActivation, ID: "slow-tools"},
			Value:  mustJSON(model.ChaosActivation{Enabled: true, ExpiresAt: &exp}),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Impact.ChaosPolicies) == 0 || plan.Impact.ChaosPolicies[0].ExpiresAt == nil {
		t.Fatalf("impact=%+v", plan.Impact.ChaosPolicies)
	}
	*plan.Impact.ChaosPolicies[0].ExpiresAt = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
	if svc.Store().Load() != boot {
		t.Fatal("plan swapped")
	}
	for _, p := range svc.Store().Load().Canonical.Spec.Chaos.Policies {
		if p.ID == "slow-tools" && p.ExpiresAt != nil {
			t.Fatal("impact ExpiresAt aliased live canonical")
		}
	}
}

func TestListAndGetZonesRecords(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	ctx := context.Background()
	zones, err := svc.ListZones(ctx, actor(), Page{})
	if err != nil {
		t.Fatal(err)
	}
	if len(zones.Zones) != 1 || zones.Zones[0].ID != "lab-zone" {
		t.Fatalf("zones=%+v", zones.Zones)
	}
	z, err := svc.GetZone(ctx, actor(), "lab-zone")
	if err != nil {
		t.Fatal(err)
	}
	z.Records[0].Values[0] = "mutated"
	live, _ := svc.GetZone(ctx, actor(), "lab-zone")
	if live.Records[0].Values[0] == "mutated" {
		t.Fatal("GetZone leaked live records")
	}
	recs, err := svc.ListRecords(ctx, actor(), "lab-zone", Page{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs.Records) != 1 || recs.Records[0].ID != "ns1-a" {
		t.Fatalf("records=%+v", recs.Records)
	}
	rec, err := svc.GetRecord(ctx, actor(), "lab-zone", "ns1-a")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Values[0] != "10.42.0.53" {
		t.Fatalf("record=%+v", rec)
	}
	_, err = svc.GetZone(ctx, actor(), "missing")
	_ = requireCode(t, err, domainerr.CodeNotFound)
}

func TestResolveAndExplain(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	out, err := svc.Resolve(ctx, actor(), ResolveIn{
		Name: "ns1.lab.example.net",
		Type: model.TypeA,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.RCode != model.RCodeNoError || len(out.Result.Answers) == 0 {
		t.Fatalf("resolve=%+v", out.Result)
	}
	exp, err := svc.Explain(ctx, actor(), ResolveIn{
		Name:   "ns1.lab.example.net.",
		Type:   model.TypeA,
		Client: netip.MustParseAddr("127.0.0.1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.Explanation == nil {
		t.Fatal("explain missing explanation")
	}
	if exp.Explanation.Revision != boot.Revision {
		t.Fatalf("explain rev=%s", exp.Explanation.Revision)
	}
	if exp.Explanation.Source != model.SourceExact {
		t.Fatalf("source=%s", exp.Explanation.Source)
	}
}

func TestResolveUseCacheDoesNotStoreFallthrough(t *testing.T) {
	path := copyNamedFixture(t, "pack-sample.yaml")
	svc, boot := mustBoot(t, path)
	c := cache.New(cache.PolicyFromSpec(boot.Canonical.Spec.Cache), nil)
	svc.cache = c
	ctx := context.Background()

	miss, err := svc.Resolve(ctx, actor(), ResolveIn{
		Name:     "other.vendor.example.",
		Type:     model.TypeA,
		UseCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !miss.Result.Fallthrough || miss.Result.RCode != model.RCodeNoError {
		t.Fatalf("overlay miss=%+v", miss.Result)
	}
	ftKey := cache.Key{
		Revision: boot.Revision,
		Name:     "other.vendor.example.",
		Type:     model.TypeA,
		Class:    model.ClassIN,
		Local:    true,
	}
	if _, ok := c.Get(ftKey, cache.GetOpts{}); ok {
		t.Fatal("management resolve must not cache overlay fallthrough")
	}

	hit, err := svc.Resolve(ctx, actor(), ResolveIn{
		Name:     "special-api.vendor.example.",
		Type:     model.TypeA,
		UseCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit.Result.Fallthrough || len(hit.Result.Answers) == 0 {
		t.Fatalf("local overlay hit=%+v", hit.Result)
	}
	localKey := cache.Key{
		Revision: boot.Revision,
		Name:     "special-api.vendor.example.",
		Type:     model.TypeA,
		Class:    model.ClassIN,
		Local:    true,
	}
	if _, ok := c.Get(localKey, cache.GetOpts{}); !ok {
		t.Fatal("complete local answer must still be cached")
	}
	again, err := svc.Resolve(ctx, actor(), ResolveIn{
		Name:     "special-api.vendor.example.",
		Type:     model.TypeA,
		UseCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.Result.Source != model.SourceCache {
		t.Fatalf("cached source=%s", again.Result.Source)
	}
}

func TestForwardingAndCacheStatus(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	c := cache.New(cache.Policy{Enabled: true, MaxEntries: 8, MinimumTTL: 0, MaximumTTL: 0}, nil)
	svc.cache = c
	ctx := context.Background()
	pols, err := svc.ListForwardingPolicies(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if pols == nil {
		t.Fatal("nil policies")
	}
	st, err := svc.CacheStatus(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled {
		t.Fatal("cache should be enabled")
	}
	if err := svc.CacheFlush(ctx, actor(), FlushIn{All: true}); err != nil {
		t.Fatal(err)
	}
	_ = boot
}

func TestVersionSchemaDocsStatus(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Version(ctx, actor()); err != nil {
		t.Fatal(err)
	}
	caps, err := svc.Capabilities(ctx, actor())
	if err != nil || len(caps.Capabilities) == 0 {
		t.Fatalf("caps=%v err=%v", caps, err)
	}
	var sawHealth, sawApply bool
	for _, c := range caps.Capabilities {
		if c.Name == "health.live" {
			sawHealth = true
		}
		if c.Name == "dns_change_apply" && c.Mutating {
			sawApply = true
		}
	}
	if !sawHealth || !sawApply {
		t.Fatalf("capabilities discovery missing health or apply: %+v", caps.Capabilities)
	}
	st, err := svc.Status(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if st.Revisions.RuntimeRevision == "" {
		t.Fatal("status missing revision")
	}
	schema, err := svc.ConfigSchema(ctx, actor())
	if err != nil || len(schema) == 0 {
		t.Fatalf("schema err=%v", err)
	}
	docs, err := svc.Docs(ctx, actor(), "dns-semantics")
	if err != nil || len(docs) == 0 {
		t.Fatalf("docs err=%v", err)
	}
	_, err = svc.Docs(ctx, actor(), "missing")
	_ = requireCode(t, err, domainerr.CodeNotFound)
}

func TestAuditRing(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	applied, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Reason:           "audit me",
		Operations:       []model.Operation{addWWWRecord()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied.AuditEventID == "" {
		t.Fatal("apply missing audit id")
	}
	ev, err := svc.GetAudit(ctx, actor(), applied.AuditEventID)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Reason != "audit me" {
		t.Fatalf("event=%+v", ev)
	}
	list, err := svc.QueryAudit(ctx, actor(), AuditQuery{Limit: 10})
	if err != nil || len(list.Events) == 0 {
		t.Fatalf("list=%v err=%v", list, err)
	}
}
