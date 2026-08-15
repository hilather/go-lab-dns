package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestRegistryEmitsCatalogMetrics(t *testing.T) {
	r := NewRegistry()
	r.Inc(MetricDNSQueries, map[string]string{
		"transport": "udp", "client_group_class": "known", "qtype_class": "A",
		"source": "exact", "rcode": "NOERROR",
	}, 1)
	r.Inc(MetricDeniedForward, map[string]string{"result": "unknown"}, 1)
	r.Observe(MetricDNSQueryDuration, map[string]string{"transport": "udp", "source": "exact"}, 0.004)
	r.Set(MetricChaosEmergency, nil, 1)
	v, ok := r.Get(MetricDNSQueries, map[string]string{
		"transport": "udp", "client_group_class": "known", "qtype_class": "A",
		"source": "exact", "rcode": "NOERROR",
	})
	if !ok || v != 1 {
		t.Fatalf("queries=%v ok=%v", v, ok)
	}
	var buf bytes.Buffer
	if err := r.WritePrometheus(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, MetricDNSQueries) || !strings.Contains(out, `rcode="NOERROR"`) {
		t.Fatalf("prometheus scrape missing query series:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "qname") || strings.Contains(out, "1.2.3.4") {
		t.Fatalf("scrape leaked qname/ip:\n%s", out)
	}
}

func TestRegistryRejectsQNAMEAndClientIP(t *testing.T) {
	r := NewRegistry()
	r.Inc(MetricDNSQueries, map[string]string{
		"transport": "udp", "qname": "evil.example.", "rcode": "NOERROR",
		"client_group_class": "known", "qtype_class": "A", "source": "exact",
	}, 1)
	if _, ok := r.Get(MetricDNSQueries, map[string]string{
		"transport": "udp", "qname": "evil.example.", "rcode": "NOERROR",
		"client_group_class": "known", "qtype_class": "A", "source": "exact",
	}); ok {
		t.Fatal("qname label must not be stored")
	}
	r.Inc(MetricDNSAdmitted, map[string]string{"transport": "udp", "client_ip": "192.0.2.1"}, 1)
	if _, ok := r.Get(MetricDNSAdmitted, map[string]string{"transport": "udp", "client_ip": "192.0.2.1"}); ok {
		t.Fatal("client_ip must not be stored")
	}
	dropped, ok := r.Get(MetricTelemetryDropped, map[string]string{"reason": "forbidden_label"})
	if !ok || dropped < 2 {
		t.Fatalf("expected forbidden_label drops, got %v ok=%v", dropped, ok)
	}
}

func TestRegistrySeriesCap(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < MaxSeriesPerMetric+10; i++ {
		r.Inc(MetricChaosEffects, map[string]string{"policy_id": "p-" + itoa(i), "action": "drop"}, 1)
	}
	n := 0
	for _, s := range r.Snapshot() {
		if s.Name == MetricChaosEffects {
			n++
		}
	}
	if n > MaxSeriesPerMetric {
		t.Fatalf("series=%d cap=%d", n, MaxSeriesPerMetric)
	}
	if r.Dropped() == 0 {
		t.Fatal("expected cardinality drops")
	}
}

func TestUnusedExportQueueDoesNotDrop(t *testing.T) {
	r := NewRegistry()
	if r.Export() != nil {
		t.Fatal("export queue must stay nil until EnableExport")
	}
	for i := 0; i < DefaultQueueSize+8; i++ {
		r.Inc(MetricDNSParse, map[string]string{"result": "ok"}, 1)
	}
	if r.Dropped() != 0 {
		t.Fatalf("unused export must not count drops, dropped=%d", r.Dropped())
	}
	if _, ok := r.Get(MetricTelemetryDropped, map[string]string{"reason": "export"}); ok {
		t.Fatal("false-positive export drop")
	}
}

func TestExportBackpressureDoesNotBlock(t *testing.T) {
	r := NewRegistry()
	q := r.EnableExport(1)
	// Fill the export queue, then Inc must still return and record.
	for i := 0; i < q.Cap()+8; i++ {
		r.Inc(MetricDNSParse, map[string]string{"result": "ok"}, 1)
	}
	v, ok := r.Get(MetricDNSParse, map[string]string{"result": "ok"})
	if !ok || v < 1 {
		t.Fatalf("counter lost under export backpressure v=%v", v)
	}
	if q.Dropped() == 0 {
		t.Fatal("expected export queue drops")
	}
}

func TestLabelEncodeDoesNotInventQNAME(t *testing.T) {
	r := NewRegistry()
	evil := "a,qname=evil.example."
	r.Inc(MetricChaosEffects, map[string]string{"policy_id": evil, "action": "drop"}, 1)
	var buf bytes.Buffer
	if err := r.WritePrometheus(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, `qname="`) {
		t.Fatalf("scrape invented qname label:\n%s", out)
	}
	if !strings.Contains(out, evil) {
		t.Fatalf("policy_id value missing from scrape:\n%s", out)
	}
}

func TestCheckLabelsUnknownMetric(t *testing.T) {
	if err := CheckLabels("not_a_metric", nil); err == nil {
		t.Fatal("expected unknown_metric")
	}
}

func TestDNSTransportAdapter(t *testing.T) {
	r := NewRegistry()
	m := NewDNSTransport(r)
	m.IncQuery("udp")
	m.IncParse("ok")
	m.IncAdmission("ok", "")
	m.IncResponse("udp", "NOERROR", "send")
	m.IncTCP("accept")
	if v, ok := r.Get(MetricDNSAdmitted, map[string]string{"transport": "udp"}); !ok || v != 1 {
		t.Fatalf("admitted=%v", v)
	}
	if v, ok := r.Get(MetricDNSResponses, map[string]string{"transport": "udp", "rcode": "NOERROR", "action": "send"}); !ok || v != 1 {
		t.Fatalf("responses=%v", v)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	n := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}
