package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/buildinfo"
)

func TestBodyTooLarge(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, MaxBodyBytes: 64})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"name":"` + strings.Repeat("a", 200) + `.lab.example.net.","type":"A"}`
	rec := doLoopback(t, s.Handler(), http.MethodPost, "/v1/resolve", body)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
}

func TestWrongContentType(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptestNewJSON(http.MethodPost, "/v1/resolve", `{"name":"x","type":"A"}`)
	req.Header.Set("Content-Type", "text/plain")
	req.RemoteAddr = "127.0.0.1:1"
	rec := httptestNewRec()
	s.Handler().ServeHTTP(rec, req)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
}

func TestRequestTimeout(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	slow := &slowVersion{App: svc}
	s, err := New(Config{Service: slow, RequestTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	rec := doLoopback(t, s.Handler(), http.MethodGet, "/v1/version", "")
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		// handler observes ctx.Err() after Version returns
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// The timeout is applied to the request context; slow Version should see cancel.
	if !slow.sawCancel {
		// Version may return after cancel; dispatch then writes internal/canceled.
		if rec.Code == http.StatusOK {
			t.Fatal("slow handler completed without observing cancel")
		}
	}
}

func TestRateLimitConcurrent(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, MaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Occupy the single slot.
	s.inflight <- struct{}{}
	rec := doLoopback(t, s.Handler(), http.MethodGet, "/v1/version", "")
	requireProblem(t, rec, http.StatusTooManyRequests, "rate_limited")
	<-s.inflight
}

type slowVersion struct {
	*app.App
	sawCancel bool
}

func (s *slowVersion) Version(ctx context.Context, actor auth.Actor) (*buildinfo.Info, error) {
	select {
	case <-ctx.Done():
		s.sawCancel = true
		return nil, ctx.Err()
	case <-time.After(200 * time.Millisecond):
		return s.App.Version(ctx, actor)
	}
}

func httptestNewJSON(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func httptestNewRec() *httptest.ResponseRecorder { return httptest.NewRecorder() }
