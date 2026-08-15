package dnsserver

import (
	"strings"
	"sync"
	"testing"
)

type spyMetrics struct {
	mu     sync.Mutex
	events []string
}

func (s *spyMetrics) add(parts ...string) {
	s.mu.Lock()
	s.events = append(s.events, strings.Join(parts, "|"))
	s.mu.Unlock()
}

func (s *spyMetrics) IncQuery(transport string) { s.add("query", transport) }
func (s *spyMetrics) IncParse(result string)    { s.add("parse", result) }
func (s *spyMetrics) IncAdmission(result, rcode string) {
	s.add("admit", result, rcode)
}
func (s *spyMetrics) IncResponse(transport, rcode, action string) {
	s.add("resp", transport, rcode, action)
}
func (s *spyMetrics) IncTCP(event string) { s.add("tcp", event) }

func (s *spyMetrics) joined() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.events, "\n")
}

func TestMetricsDoNotIncludeQNAME(t *testing.T) {
	spy := &spyMetrics{}
	s := startServer(t, Config{
		Handler: StaticA("192.0.2.1"),
		Metrics: spy,
	})
	secret := "do-not-log.example."
	_ = mustExchangeUDP(t, s.UDPAddr(), packA(t, secret, 99, nil))
	_ = mustExchangeTCP(t, s.TCPAddr(), packA(t, secret, 100, nil))
	dump := spy.joined()
	if dump == "" {
		t.Fatal("no metrics recorded")
	}
	if strings.Contains(dump, "do-not-log") || strings.Contains(dump, secret) {
		t.Fatalf("QNAME leaked into metrics:\n%s", dump)
	}
	if !strings.Contains(dump, "udp") || !strings.Contains(dump, "tcp") {
		t.Fatalf("missing transport labels:\n%s", dump)
	}
}
