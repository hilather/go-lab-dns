package rest

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
)

func TestSessionCookieFlags(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doLoopback(t, s.Handler(), http.MethodPost, "/v1/session", "")
	requireStatus(t, rec, http.StatusOK)
	c := sessionCookie(t, rec)
	if !c.HttpOnly {
		t.Fatal("HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite=%v", c.SameSite)
	}
	if c.Path != "/" {
		t.Fatalf("Path=%q", c.Path)
	}
	if c.Domain != "" {
		t.Fatalf("Domain=%q", c.Domain)
	}
	if c.Secure {
		t.Fatal("Secure set without TLS")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control=%q", rec.Header().Get("Cache-Control"))
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/session", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.TLS = &tls.ConnectionState{}
	tlsRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(tlsRec, req)
	requireStatus(t, tlsRec, http.StatusOK)
	if !sessionCookie(t, tlsRec).Secure {
		t.Fatal("Secure not set with TLS")
	}
}

func TestSessionCSRFFirstLoginOmitsThenRequires(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	first := doLoopback(t, h, http.MethodPost, "/v1/session", "")
	requireStatus(t, first, http.StatusOK)
	csrf := decodeJSON(t, first)["csrf"].(string)
	id := sessionCookie(t, first).Value

	noCSRF := doSession(t, h, http.MethodPost, "/v1/session", "127.0.0.1:9", id, "", "")
	requireProblem(t, noCSRF, http.StatusForbidden, "forbidden")

	bad := doSession(t, h, http.MethodPost, "/v1/session", "127.0.0.1:9", id, "deadbeef", "")
	requireProblem(t, bad, http.StatusForbidden, "forbidden")

	ok := doSession(t, h, http.MethodPost, "/v1/session", "127.0.0.1:9", id, csrf, "")
	requireStatus(t, ok, http.StatusOK)
	rot := sessionCookie(t, ok)
	if rot.Value == id {
		t.Fatal("did not rotate id")
	}
	if decodeJSON(t, ok)["csrf"].(string) == csrf {
		t.Fatal("did not rotate csrf")
	}
}

func TestSessionBearerWinsIgnoresCookieAndCSRF(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	pol, err := auth.NewPolicy(auth.PolicyConfig{Tokens: []auth.Token{
		{Token: "view", ID: "viewer", Role: auth.RoleViewer},
		{Token: "adm", ID: "admin", Role: auth.RoleAdministrator},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: pol, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	first := doSession(t, h, http.MethodPost, "/v1/session", "192.0.2.10:9", "", "", "view")
	requireStatus(t, first, http.StatusOK)
	id := sessionCookie(t, first).Value
	if got := actorClass(t, first); got != auth.ClassUISession {
		t.Fatalf("class=%s", got)
	}

	switched := doSession(t, h, http.MethodPost, "/v1/session", "192.0.2.10:9", id, "", "adm")
	requireStatus(t, switched, http.StatusOK)
	actor := decodeJSON(t, switched)["actor"].(map[string]any)
	if actor["id"] != "admin" {
		t.Fatalf("actor=%v", actor)
	}
	if actor["role"] != auth.RoleAdministrator {
		t.Fatalf("role=%v", actor)
	}
}

func TestSessionLoopbackCreate(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doLoopback(t, s.Handler(), http.MethodPost, "/v1/session", "")
	requireStatus(t, rec, http.StatusOK)
	actor := decodeJSON(t, rec)["actor"].(map[string]any)
	if actor["id"] != "loopback" || actor["class"] != auth.ClassUISession || actor["role"] != auth.RoleAdministrator {
		t.Fatalf("actor=%v", actor)
	}
	scopes, _ := actor["scopes"].([]any)
	if len(scopes) != len(auth.AllScopes()) {
		t.Fatalf("scopes=%v", scopes)
	}
	got := doSession(t, s.Handler(), http.MethodGet, "/v1/session", "127.0.0.1:9", sessionCookie(t, rec).Value, "", "")
	requireStatus(t, got, http.StatusOK)
	if decodeJSON(t, got)["csrf"] != decodeJSON(t, rec)["csrf"] {
		t.Fatal("GET csrf mismatch")
	}
	anon := doLoopback(t, s.Handler(), http.MethodGet, "/v1/session", "")
	requireProblem(t, anon, http.StatusUnauthorized, "unauthenticated")
}

func TestSessionLoopbackViewerRotateDoesNotEscalate(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	pol, err := auth.NewPolicy(auth.PolicyConfig{Tokens: []auth.Token{
		{Token: "view", ID: "viewer", Role: auth.RoleViewer},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: pol, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	first := doSession(t, h, http.MethodPost, "/v1/session", "127.0.0.1:9", "", "", "view")
	requireStatus(t, first, http.StatusOK)
	csrf := decodeJSON(t, first)["csrf"].(string)
	id := sessionCookie(t, first).Value
	rot := doSession(t, h, http.MethodPost, "/v1/session", "127.0.0.1:9", id, csrf, "")
	requireStatus(t, rot, http.StatusOK)
	actor := decodeJSON(t, rot)["actor"].(map[string]any)
	if actor["id"] != "viewer" || actor["role"] != auth.RoleViewer || actor["class"] != auth.ClassUISession {
		t.Fatalf("actor=%v", actor)
	}
	for _, sc := range actor["scopes"].([]any) {
		if sc == auth.ScopeDNSAdmin {
			t.Fatal("viewer session gained dns.admin")
		}
	}
}

func TestSessionGETRootNever401(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	for _, remote := range []string{"127.0.0.1:9", "192.0.2.10:9"} {
		for _, path := range []string{"/", "/login", "/assets/app.js"} {
			rec := doRemote(t, h, http.MethodGet, path, "", remote, "")
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("%s %s was 401", remote, path)
			}
			requireProblem(t, rec, http.StatusNotFound, "not_found")
			if rec.Header().Get("Content-Security-Policy") == "" {
				t.Fatalf("missing CSP on %s", path)
			}
			if rec.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("CORS header")
			}
		}
	}
	state := doRemote(t, h, http.MethodGet, "/v1/state", "", "192.0.2.10:9", "")
	requireProblem(t, state, http.StatusUnauthorized, "unauthenticated")
}

func TestSessionUIDisabledOrNilIs404Not401(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	ui := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>spa</html>"))
	})
	disabled, err := New(Config{Service: svc, RatePerSec: -1, UI: ui, UIEnabled: func() bool { return false }})
	if err != nil {
		t.Fatal(err)
	}
	rec := doRemote(t, disabled.Handler(), http.MethodGet, "/", "", "192.0.2.10:9", "")
	requireProblem(t, rec, http.StatusNotFound, "not_found")
	st := doRemote(t, disabled.Handler(), http.MethodGet, "/v1/state", "", "127.0.0.1:9", "")
	requireStatus(t, st, http.StatusOK)

	enabled, err := New(Config{Service: svc, RatePerSec: -1, UI: ui})
	if err != nil {
		t.Fatal(err)
	}
	html := doRemote(t, enabled.Handler(), http.MethodGet, "/", "", "192.0.2.10:9", "")
	requireStatus(t, html, http.StatusOK)
	if !strings.Contains(html.Body.String(), "spa") {
		t.Fatalf("body=%s", html.Body.String())
	}
	if html.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}

func TestSessionSecurityHeadersOnAPI(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doLoopback(t, s.Handler(), http.MethodGet, "/v1/version", "")
	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("frame")
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("referrer")
	}
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP on JSON")
	}
}

func TestSessionDeleteRequiresCSRF(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	first := doLoopback(t, h, http.MethodPost, "/v1/session", "")
	requireStatus(t, first, http.StatusOK)
	id := sessionCookie(t, first).Value
	csrf := decodeJSON(t, first)["csrf"].(string)
	no := doSession(t, h, http.MethodDelete, "/v1/session", "127.0.0.1:9", id, "", "")
	requireProblem(t, no, http.StatusForbidden, "forbidden")
	del := doSession(t, h, http.MethodDelete, "/v1/session", "127.0.0.1:9", id, csrf, "")
	requireStatus(t, del, http.StatusNoContent)
	if !strings.Contains(del.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("Set-Cookie=%q", del.Header().Get("Set-Cookie"))
	}
	got := doSession(t, h, http.MethodGet, "/v1/session", "127.0.0.1:9", id, "", "")
	requireProblem(t, got, http.StatusUnauthorized, "unauthenticated")
}

func TestSessionTableFullREST(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	tab := auth.NewSessionTable(auth.SessionTableConfig{Max: 1})
	s, err := New(Config{Service: svc, Sessions: tab, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	first := doLoopback(t, h, http.MethodPost, "/v1/session", "")
	requireStatus(t, first, http.StatusOK)
	csrf := decodeJSON(t, first)["csrf"].(string)
	id := sessionCookie(t, first).Value
	rot := doSession(t, h, http.MethodPost, "/v1/session", "127.0.0.1:9", id, csrf, "")
	requireStatus(t, rot, http.StatusOK)
	full := doLoopback(t, h, http.MethodPost, "/v1/session", "")
	m := requireProblem(t, full, http.StatusTooManyRequests, "rate_limited")
	if m["detail"] != "session table full" {
		t.Fatalf("detail=%v", m["detail"])
	}
}

func TestSessionOriginsFromClosure(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{
		Service:    svc,
		RatePerSec: -1,
		Origins:    func() []string { return []string{"https://dns-mgmt.lab.example"} },
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("Origin", "https://dns-mgmt.lab.example")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)
	req.Header.Set("Origin", "https://evil.example")
	bad := httptest.NewRecorder()
	s.Handler().ServeHTTP(bad, req)
	requireProblem(t, bad, http.StatusForbidden, "forbidden")
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatalf("missing %s cookie: %v", auth.CookieName, rec.Header().Get("Set-Cookie"))
	return nil
}

func actorClass(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	actor, _ := decodeJSON(t, rec)["actor"].(map[string]any)
	s, _ := actor["class"].(string)
	return s
}

func doSession(t *testing.T, h http.Handler, method, path, remote, cookie, csrf, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remote
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	}
	if csrf != "" {
		req.Header.Set(auth.CSRFHeader, csrf)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
