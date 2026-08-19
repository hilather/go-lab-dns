package mcp

import (
	"net/http"
	"testing"
)

func TestOriginMissingAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}, "127.0.0.1:1")
	if rec.Code == http.StatusForbidden {
		t.Fatalf("missing Origin rejected: %s", rec.Body.String())
	}
}

func TestOriginLoopbackAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	for _, origin := range []string{"http://127.0.0.1:8080", "http://localhost", "http://[::1]:9"} {
		rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
			"Content-Type":        "application/json",
			"Accept":              "application/json, text/event-stream",
			headerProtocolVersion: ProtocolVersion,
			headerOrigin:          origin,
		}, "127.0.0.1:1")
		if rec.Code == http.StatusForbidden {
			t.Fatalf("loopback Origin %s rejected: %s", origin, rec.Body.String())
		}
	}
}

func TestOriginRemoteDenied(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://evil.example",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusForbidden, "forbidden")
}

func TestOriginAllowlist(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, AllowedOrigins: []string{"https://mgmt.lab.example"}})
	if err != nil {
		t.Fatal(err)
	}
	ok := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://mgmt.lab.example",
	}, "127.0.0.1:1")
	if ok.Code == http.StatusForbidden {
		t.Fatalf("allowlisted Origin rejected: %s", ok.Body.String())
	}
	bad := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://other.example",
	}, "127.0.0.1:1")
	requireRPCError(t, bad, http.StatusForbidden, "forbidden")
}

func TestOriginFuncAllowlist(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, Origins: func() []string { return []string{"https://mgmt.lab.example"} }})
	if err != nil {
		t.Fatal(err)
	}
	ok := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://mgmt.lab.example",
	}, "127.0.0.1:1")
	if ok.Code == http.StatusForbidden {
		t.Fatalf("Origins func allowlist rejected: %s", ok.Body.String())
	}
}

func TestOriginAllowedHelper(t *testing.T) {
	if originAllowed("https://evil.example", nil) {
		t.Fatal("evil origin allowed")
	}
	if !originAllowed("http://127.0.0.1", nil) {
		t.Fatal("loopback origin denied")
	}
	if originAllowed("file://localhost", nil) {
		t.Fatal("file origin allowed")
	}
	if !originAllowed("https://ok.example", []string{"https://ok.example"}) {
		t.Fatal("allowlist miss")
	}
}
