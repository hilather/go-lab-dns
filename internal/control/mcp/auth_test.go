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
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	pol, err := auth.NewPolicy(auth.PolicyConfig{
		Profile: auth.ProfileBearer,
		Tokens:  []auth.Token{{Token: "dev-token", Role: auth.RoleAdministrator}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: pol, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
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

func TestNilAuthenticatorFailsClosed(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	body := rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	})
	loop := doRaw(t, s.Handler(), body, mcpAuthHdr("tools/call", "dns_version_get", ""), "127.0.0.1:9")
	requireRPCError(t, loop, http.StatusUnauthorized, "unauthenticated")
	bearer := doRaw(t, s.Handler(), body, mcpAuthHdr("tools/call", "dns_version_get", "dev-token"), "192.0.2.10:9")
	requireRPCError(t, bearer, http.StatusUnauthorized, "unauthenticated")
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

func TestRemoteCookieDoesNotAuthenticate(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	}), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		"Cookie":              auth.CookieName + "=deadbeef",
	}, "192.0.2.10:9")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
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
	meta := map[string]any{
		"io.modelcontextprotocol/protocolVersion": ProtocolVersion,
		"io.modelcontextprotocol/clientInfo": map[string]any{
			"name": "bearer-loopback", "version": "dev",
		},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
	// Protocol 2026-07-28 removed JSON-RPC "initialize"; the handshake is
	// server/discover. A bare initialize is still an auth gate: under bearer
	// it must 401 before the SDK reports the method removed.
	initBody := rpcCall(1, "initialize", map[string]any{"_meta": meta})
	discoverBody := rpcCall(1, "server/discover", map[string]any{"_meta": meta})
	toolBody := rpcCall(2, "tools/call", map[string]any{
		"_meta":     meta,
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	})
	for _, remote := range []string{"127.0.0.1:9", "[::1]:9"} {
		requireRPCError(t, doRaw(t, s.Handler(), toolBody, mcpAuthHdr("tools/call", "dns_version_get", ""), remote), http.StatusUnauthorized, "unauthenticated")
		requireRPCError(t, doRaw(t, s.Handler(), discoverBody, mcpAuthHdr("server/discover", "", ""), remote), http.StatusUnauthorized, "unauthenticated")
		requireRPCError(t, doRaw(t, s.Handler(), initBody, mcpAuthHdr("initialize", "", ""), remote), http.StatusUnauthorized, "unauthenticated")
	}
	discoverOK := doRaw(t, s.Handler(), discoverBody, mcpAuthHdr("server/discover", "", "good"), "127.0.0.1:9")
	if discoverOK.Code != http.StatusOK && discoverOK.Code != http.StatusAccepted {
		t.Fatalf("discover with bearer status=%d body=%s", discoverOK.Code, discoverOK.Body.String())
	}
	if decodeRPC(t, discoverOK)["result"] == nil {
		t.Fatalf("discover result: %s", discoverOK.Body.String())
	}
	toolOK := doRaw(t, s.Handler(), toolBody, mcpAuthHdr("tools/call", "dns_version_get", "good"), "127.0.0.1:9")
	if toolOK.Code == http.StatusUnauthorized || decodeRPC(t, toolOK)["result"] == nil {
		t.Fatalf("tools/call with bearer status=%d body=%s", toolOK.Code, toolOK.Body.String())
	}

	dev, err := auth.NewPolicy(auth.PolicyConfig{Profile: auth.ProfileDevLoopbackUnauth})
	if err != nil {
		t.Fatal(err)
	}
	devSrv, err := New(Config{Service: svc, Auth: dev, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	devRec := doRaw(t, devSrv.Handler(), toolBody, mcpAuthHdr("tools/call", "dns_version_get", ""), "127.0.0.1:9")
	if devRec.Code == http.StatusUnauthorized || decodeRPC(t, devRec)["result"] == nil {
		t.Fatalf("dev-loopback-unauth status=%d body=%s", devRec.Code, devRec.Body.String())
	}
}

func mcpAuthHdr(method, name, bearer string) map[string]string {
	h := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		"Mcp-Method":          method,
	}
	if name != "" {
		h["Mcp-Name"] = name
	}
	if bearer != "" {
		h[headerAuthorization] = "Bearer " + bearer
	}
	return h
}

func TestIsLoopback(t *testing.T) {
	if !isLoopback("127.0.0.1:1") || !isLoopback("[::1]:80") {
		t.Fatal("loopback not detected")
	}
	if isLoopback("192.0.2.1:1") || isLoopback("10.0.0.1:8080") {
		t.Fatal("remote treated as loopback")
	}
}
