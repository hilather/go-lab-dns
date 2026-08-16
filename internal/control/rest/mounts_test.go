package rest

import (
	"net/http"
	"testing"
)

func TestMountsServeAlongsideREST(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	mounted := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("mounted"))
	})
	s, err := New(Config{
		Service:    svc,
		RatePerSec: -1,
		Mounts:     map[string]http.Handler{"/mcp": mounted},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := doLoopback(t, s.Handler(), http.MethodPost, "/mcp", "")
	if rec.Code != http.StatusTeapot || rec.Body.String() != "mounted" {
		t.Fatalf("mount not served: status=%d body=%q", rec.Code, rec.Body.String())
	}

	// REST routing is unaffected: known route works, unknown path still 404s.
	rec = doLoopback(t, s.Handler(), http.MethodGet, "/v1/state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("/v1/state status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doLoopback(t, s.Handler(), http.MethodGet, "/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/nope status=%d want 404", rec.Code)
	}
}

func TestNoMountsUnchanged(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doLoopback(t, s.Handler(), http.MethodPost, "/mcp", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/mcp without mounts status=%d want 404", rec.Code)
	}
}
