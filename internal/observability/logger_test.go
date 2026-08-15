package observability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLoggerRedactsQNAMEAndClient(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf).WithSync()
	l.Log(Record{
		Event:  EventDNSQuery,
		QNAME:  "secret.lab.example.net.",
		Client: "192.0.2.55",
		ZoneID: "lab-zone",
	})
	s := buf.String()
	if strings.Contains(s, "secret.lab") || strings.Contains(s, "192.0.2.55") {
		t.Fatalf("leaked sensitive fields: %s", s)
	}
	var rec Record
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Event != EventDNSQuery || rec.ZoneID != "lab-zone" {
		t.Fatalf("record=%+v", rec)
	}
}

func TestLoggerLogQNAMEDebug(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf).WithSync()
	l.LogQNAME = true
	l.Log(Record{Event: EventDNSQuery, QNAME: "debug.example.", Timestamp: time.Unix(0, 0).UTC()})
	if !strings.Contains(buf.String(), "debug.example.") {
		t.Fatalf("debug mode should keep qname: %s", buf.String())
	}
}

func TestLoggerQueueDropDoesNotBlock(t *testing.T) {
	l := NewLogger(nil)
	l.q = NewQueue[Record](1)
	l.q.TrySend(Record{Event: EventDNSQuery})
	done := make(chan struct{})
	go func() {
		l.Log(Record{Event: EventDeniedForward})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Log blocked on full queue")
	}
	if l.Queue().Dropped() == 0 {
		t.Fatal("expected drop")
	}
}
