package rest

import (
	"net/http"
	"testing"

	"github.com/hilather/go-lab-dns/internal/observability"
)

func TestCapabilityMetricsAndStatusDTO(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	reg := observability.NewRegistry()
	s, err := New(Config{Service: svc, Metrics: reg, Logger: observability.NewLogger(nil).WithSync()})
	if err != nil {
		t.Fatal(err)
	}
	st := doLoopback(t, s.Handler(), http.MethodGet, "/v1/status", "")
	requireStatus(t, st, http.StatusOK)
	body := decodeJSON(t, st)
	if body["ready"] != true {
		t.Fatalf("status=%s", st.Body.String())
	}
	if body["revisions"] == nil || body["listeners"] == nil || body["chaos"] == nil {
		t.Fatalf("incomplete status %s", st.Body.String())
	}
	v, ok := reg.Get(observability.MetricCapabilityCalls, map[string]string{
		"capability": "status", "transport": "rest", "result": "ok",
	})
	if !ok || v < 1 {
		t.Fatalf("capability metric=%v ok=%v snap=%v", v, ok, reg.Snapshot())
	}
	denied := doRemote(t, s.Handler(), http.MethodGet, "/v1/status", "", "192.0.2.8:1", "")
	requireProblem(t, denied, http.StatusUnauthorized, "unauthenticated")
	if av, ok := reg.Get(observability.MetricAuthFailures, map[string]string{"result": "unauthenticated"}); !ok || av < 1 {
		t.Fatalf("auth failures=%v ok=%v", av, ok)
	}
}
