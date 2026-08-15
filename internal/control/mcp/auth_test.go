package mcp

import (
	"context"
	"net/http"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestLoopbackUnauthenticatedAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	res := callTool(t, cs, "dns_version_get", map[string]any{})
	if res.IsError {
		t.Fatalf("loopback version: %+v", res)
	}
}

func TestRemoteUnauthenticatedDenied(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	}), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}, "192.0.2.10:9")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
}

func TestRemoteBearerAccepted(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	}), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Bearer dev-token",
	}, "192.0.2.10:9")
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("bearer rejected: %s", rec.Body.String())
	}
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
	hdr := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Bearer nope",
	}
	body := rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	})
	bad := doRaw(t, s.Handler(), body, hdr, "192.0.2.10:9")
	requireRPCError(t, bad, http.StatusUnauthorized, "unauthenticated")
	hdr[headerAuthorization] = "Bearer good"
	good := doRaw(t, s.Handler(), body, hdr, "192.0.2.10:9")
	if good.Code == http.StatusUnauthorized {
		t.Fatalf("good token rejected: %s", good.Body.String())
	}
}

func TestRemoteXForwardedForNotTrusted(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		"X-Forwarded-For":     "127.0.0.1",
		"X-Real-IP":           "127.0.0.1",
	}, "192.0.2.10:9")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func TestIsLoopback(t *testing.T) {
	if !isLoopback("127.0.0.1:1") || !isLoopback("[::1]:80") {
		t.Fatal("loopback not detected")
	}
	if isLoopback("192.0.2.1:1") || isLoopback("10.0.0.1:8080") {
		t.Fatal("remote treated as loopback")
	}
}
