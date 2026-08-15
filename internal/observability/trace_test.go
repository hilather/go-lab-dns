package observability

import (
	"context"
	"strings"
	"testing"
)

func TestTraceRedaction(t *testing.T) {
	attrs := RedactAttrs(map[string]string{
		"qname":     "hidden.example.",
		"client_ip": "192.0.2.9",
		"zone_id":   "lab-zone",
		"transport": "udp",
	}, false)
	if _, ok := attrs["qname"]; ok {
		t.Fatal("qname must be omitted")
	}
	if _, ok := attrs["client_ip"]; ok {
		t.Fatal("client_ip must be omitted")
	}
	if attrs["zone_id"] != "lab-zone" || attrs["transport"] != "udp" {
		t.Fatalf("kept attrs=%v", attrs)
	}
	hashed := RedactAttrs(map[string]string{"qname": "hidden.example."}, true)
	if !strings.HasPrefix(hashed["qname_hash"], "sha256:") {
		t.Fatalf("hash=%v", hashed)
	}
	if strings.Contains(hashed["qname_hash"], "hidden") {
		t.Fatal("hash leaked name")
	}
}

func TestTracerSampledAndUnsampled(t *testing.T) {
	off := NewTracer(0, nil)
	ctx, sp := off.Start(context.Background(), "dns.receive", map[string]string{"qname": "x."})
	if sp.Sampled {
		t.Fatal("rate 0 must not sample")
	}
	off.Finish(sp)
	if off.Queue().Len() != 0 {
		t.Fatal("unsampled span must not enqueue")
	}
	_ = ctx

	on := NewTracer(1, nil)
	_, sp = on.Start(WithTraceID(context.Background(), "abc"), "capability.invoke", map[string]string{"capability": "dns_status_get"})
	if !sp.Sampled || sp.TraceID != "abc" {
		t.Fatalf("span=%+v", sp)
	}
	on.Finish(sp)
	if on.Queue().Len() != 1 {
		t.Fatal("sampled span should enqueue")
	}
}

func TestRequestIDContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	ctx = WithTraceID(ctx, "tr-1")
	if RequestIDFrom(ctx) != "req-1" || TraceIDFrom(ctx) != "tr-1" {
		t.Fatalf("ids request=%s trace=%s", RequestIDFrom(ctx), TraceIDFrom(ctx))
	}
}
