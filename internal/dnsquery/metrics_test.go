package dnsquery

import (
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/observability"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestQueryEmitsBoundedMetrics(t *testing.T) {
	st := loadPack(t)
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	reg := observability.NewRegistry()
	h := NewOpts(Opts{Store: store, Metrics: reg})

	res := serve(t, h, model.Query{
		Name:      "ns1.lab.example.net.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("10.42.0.10"),
		Transport: model.TransportUDP,
	})
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	v, ok := reg.Get(observability.MetricDNSQueries, map[string]string{
		"transport":          "udp",
		"client_group_class": "known",
		"qtype_class":        "A",
		"source":             "exact",
		"rcode":              "NOERROR",
	})
	if !ok || v < 1 {
		t.Fatalf("queries=%v ok=%v samples=%v", v, ok, reg.Snapshot())
	}
	if _, ok := reg.Get(observability.MetricResolverOutcomes, map[string]string{"source": "exact", "zone_id": "lab-zone"}); !ok {
		t.Fatal("missing resolver outcome")
	}
	for _, s := range reg.Snapshot() {
		if _, bad := s.Labels["qname"]; bad {
			t.Fatalf("qname label leaked: %+v", s)
		}
		if _, bad := s.Labels["client_ip"]; bad {
			t.Fatalf("client_ip label leaked: %+v", s)
		}
	}
}

func TestDeniedForwardMetric(t *testing.T) {
	st := loadPack(t)
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	reg := observability.NewRegistry()
	h := NewOpts(Opts{Store: store, Metrics: reg})
	res := serve(t, h, model.Query{
		Name:      "www.unmatched.example.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("203.0.113.9"),
		Transport: model.TransportUDP,
		RD:        true,
	})
	if res.RCode != model.RCodeRefused {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if v, ok := reg.Get(observability.MetricDeniedForward, map[string]string{"result": "unknown"}); !ok || v < 1 {
		t.Fatalf("denied=%v ok=%v", v, ok)
	}
}
