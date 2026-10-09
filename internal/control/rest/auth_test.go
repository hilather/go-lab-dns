package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestLoopbackUnauthenticatedAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRemote(t, s.Handler(), http.MethodGet, "/v1/version", "", "127.0.0.1:9", "")
	requireStatus(t, rec, http.StatusOK)
	rec6 := doRemote(t, s.Handler(), http.MethodGet, "/v1/version", "", "[::1]:9", "")
	requireStatus(t, rec6, http.StatusOK)
}

func TestRemoteUnauthenticatedDenied(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRemote(t, s.Handler(), http.MethodGet, "/v1/version", "", "192.0.2.10:9", "")
	requireProblem(t, rec, http.StatusUnauthorized, "unauthenticated")
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
}

func TestRemoteBearerAccepted(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRemote(t, s.Handler(), http.MethodGet, "/v1/version", "", "192.0.2.10:9", "dev-token")
	requireStatus(t, rec, http.StatusOK)
}

func TestRemoteBearerRejectedByHook(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{
		Service: svc,
		Auth: AuthenticatorFunc(func(ctx context.Context, token string) (auth.Actor, error) {
			_ = ctx
			if token != "good" {
				return auth.Actor{}, domainerr.Unauthenticated("bad token")
			}
			return auth.Actor{ID: "ok", Class: "token"}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	bad := doRemote(t, s.Handler(), http.MethodGet, "/v1/version", "", "192.0.2.10:9", "nope")
	requireProblem(t, bad, http.StatusUnauthorized, "unauthenticated")
	good := doRemote(t, s.Handler(), http.MethodGet, "/v1/version", "", "192.0.2.10:9", "good")
	requireStatus(t, good, http.StatusOK)
}

func TestHealthSkipsAuth(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRemote(t, s.Handler(), http.MethodGet, "/v1/health/live", "", "192.0.2.10:9", "")
	requireStatus(t, rec, http.StatusOK)
	ready := doRemote(t, s.Handler(), http.MethodGet, "/v1/health/ready", "", "192.0.2.10:9", "")
	requireStatus(t, ready, http.StatusOK)
}

func TestRemoteXForwardedForNotTrusted(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.RemoteAddr = "192.0.2.10:9"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.Header.Set("X-Real-IP", "127.0.0.1")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	requireProblem(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func TestBearerProfileLoopbackRequiresToken(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	bearer, err := auth.NewPolicy(auth.PolicyConfig{
		Profile: auth.ProfileBearer,
		Tokens:  []auth.Token{{Token: "good", Role: auth.RoleAdministrator}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: bearer, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	for _, remote := range []string{"127.0.0.1:9", "[::1]:9", "[::ffff:127.0.0.1]:9"} {
		rec := doRemote(t, h, http.MethodGet, "/v1/state", "", remote, "")
		requireProblem(t, rec, http.StatusUnauthorized, "unauthenticated")
	}
	ok := doRemote(t, h, http.MethodGet, "/v1/state", "", "127.0.0.1:9", "good")
	requireStatus(t, ok, http.StatusOK)
	live := doRemote(t, h, http.MethodGet, "/v1/health/live", "", "127.0.0.1:9", "")
	requireStatus(t, live, http.StatusOK)
	ready := doRemote(t, h, http.MethodGet, "/v1/health/ready", "", "127.0.0.1:9", "")
	requireStatus(t, ready, http.StatusOK)
	sess := doRemote(t, h, http.MethodPost, "/v1/session", "", "127.0.0.1:9", "")
	requireProblem(t, sess, http.StatusUnauthorized, "unauthenticated")

	dev, err := auth.NewPolicy(auth.PolicyConfig{Profile: auth.ProfileDevLoopbackUnauth})
	if err != nil {
		t.Fatal(err)
	}
	devSrv, err := New(Config{Service: svc, Auth: dev, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	devRec := doRemote(t, devSrv.Handler(), http.MethodGet, "/v1/state", "", "127.0.0.1:9", "")
	requireStatus(t, devRec, http.StatusOK)
}

func TestIsLoopback(t *testing.T) {
	if !isLoopback("127.0.0.1:1") || !isLoopback("[::1]:80") {
		t.Fatal("loopback not detected")
	}
	if isLoopback("192.0.2.1:1") || isLoopback("10.0.0.1:8080") {
		t.Fatal("remote treated as loopback")
	}
}
