package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestRingBoundsAndOrder(t *testing.T) {
	r := NewRing(2)
	r.Append(Event{Reason: "a"})
	r.Append(Event{Reason: "b"})
	r.Append(Event{Reason: "c"})
	if r.Len() != 2 {
		t.Fatalf("len=%d", r.Len())
	}
	list := r.List(10)
	if len(list) != 2 || list[0].Reason != "c" || list[1].Reason != "b" {
		t.Fatalf("%+v", list)
	}
	ev, ok := r.Get(list[0].ID)
	if !ok || ev.Reason != "c" {
		t.Fatalf("%+v %v", ev, ok)
	}
}

func TestFanoutHookFailureDoesNotFailClosed(t *testing.T) {
	f := NewFanout(8, SinkFunc(func(context.Context, Event) error {
		return errors.New("sink down")
	}))
	ev := f.Record(context.Background(), Event{
		Time:   time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		Reason: "ok", Result: ResultOK, Revision: model.Revision("sha256:x"),
	})
	if ev.ID == "" {
		t.Fatal("missing id")
	}
	if f.DeliveryFailures() != 1 {
		t.Fatalf("failures=%d", f.DeliveryFailures())
	}
	if _, ok := f.Get(ev.ID); !ok {
		t.Fatal("ring missing event after hook failure")
	}
}

func TestRedactEventSecrets(t *testing.T) {
	ev := RedactEvent(Event{
		Reason: "Bearer super-secret",
		Diff: []RedactedEntry{{
			Path:  "spec.management.auth",
			Op:    "update",
			After: []byte(`{"secretRef":"/run/secrets/token"}`),
		}},
	})
	if ev.Reason != redacted {
		t.Fatalf("reason=%q", ev.Reason)
	}
	if string(ev.Diff[0].After) == `{"secretRef":"/run/secrets/token"}` {
		t.Fatalf("diff leaked: %s", ev.Diff[0].After)
	}
}
