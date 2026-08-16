package dnsserver

import (
	"strings"
	"testing"
	"time"
)

func TestQueryRateLimitDropsUDP(t *testing.T) {
	spy := &spyMetrics{}
	s := startServer(t, Config{
		Handler:         StaticA("192.0.2.1"),
		Metrics:         spy,
		QueryRatePerSec: 1,
		QueryBurst:      1,
	})
	raw := packA(t, "svc.lab.", 1, nil)
	first := mustExchangeUDP(t, s.UDPAddr(), raw)
	if len(first) == 0 {
		t.Fatal("first query dropped")
	}
	second := exchangeUDP(t, s.UDPAddr(), packA(t, "svc.lab.", 2, nil), 200*time.Millisecond)
	if len(second) != 0 {
		t.Fatalf("second query answered: %x", second)
	}
	dump := spy.joined()
	if !strings.Contains(dump, "admit|rate") {
		t.Fatalf("missing rate admission: %s", dump)
	}
}
