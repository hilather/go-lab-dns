package app

import (
	"context"
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

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
	requireCode(t, err, domainerr.CodeNotFound)
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
	requireCode(t, err, domainerr.CodeNotFound)
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
