package rest

import (
	"net/http"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestRESTRoleMatrixSharedWithAuth(t *testing.T) {
	pol, err := auth.NewPolicy(auth.PolicyConfig{Tokens: []auth.Token{
		{Token: "viewer", ID: "v", Role: auth.RoleViewer},
		{Token: "editor", ID: "e", Role: auth.RoleDNSEditor},
		{Token: "admin", ID: "a", Role: auth.RoleAdministrator},
	}})
	if err != nil {
		t.Fatal(err)
	}
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, Auth: pol, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	remote := "192.0.2.10:9"

	rec := doRemote(t, h, http.MethodGet, "/v1/version", "", remote, "viewer")
	requireStatus(t, rec, http.StatusOK)
	rec = doRemote(t, h, http.MethodGet, "/v1/forwarding/policies", "", remote, "editor")
	requireProblem(t, rec, http.StatusForbidden, "forbidden")
	rec = doRemote(t, h, http.MethodPost, "/v1/state:reset", `{"reason":"no"}`, remote, "editor")
	requireProblem(t, rec, http.StatusForbidden, "forbidden")
	rec = doRemote(t, h, http.MethodGet, "/v1/audit", "", remote, "viewer")
	requireProblem(t, rec, http.StatusForbidden, "forbidden")
	rec = doRemote(t, h, http.MethodGet, "/v1/audit", "", remote, "admin")
	requireStatus(t, rec, http.StatusOK)
}

func TestRESTOriginDeniedAndCORS(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptestNewJSON(http.MethodGet, "/v1/version", "")
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptestNewRec()
	s.Handler().ServeHTTP(rec, req)
	requireProblem(t, rec, http.StatusForbidden, "forbidden")
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS header")
	}
	opt := httptestNewJSON(http.MethodOptions, "/v1/version", "")
	opt.RemoteAddr = "127.0.0.1:1"
	orec := httptestNewRec()
	s.Handler().ServeHTTP(orec, opt)
	requireProblem(t, orec, http.StatusForbidden, "forbidden")
}

func TestRESTManagementRateLimit(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, RatePerSec: 1, RateBurst: 1})
	if err != nil {
		t.Fatal(err)
	}
	ok := doLoopback(t, s.Handler(), http.MethodGet, "/v1/version", "")
	requireStatus(t, ok, http.StatusOK)
	denied := doLoopback(t, s.Handler(), http.MethodGet, "/v1/version", "")
	requireProblem(t, denied, http.StatusTooManyRequests, "rate_limited")
}

func TestRESTSharedAuthorizerMatchesMCP(t *testing.T) {
	// Same Policy + same capability decision for REST and the shared authorizer.
	viewer := auth.Actor{ID: "v", Class: auth.ClassToken, Role: auth.RoleViewer}
	if err := auth.AuthorizeCapability(viewer, []string{auth.ScopeDNSRead}, "version"); err != nil {
		t.Fatal(err)
	}
	if err := auth.AuthorizeCapability(viewer, []string{auth.ScopeDNSAdmin}, "state.reset"); err == nil {
		t.Fatal("viewer reset")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeForbidden {
		t.Fatalf("%v", err)
	}
}
