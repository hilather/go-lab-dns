package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/buildinfo"
)

func TestProtocolVersionRequired(t *testing.T) {
	s, _ := newTestServer(t)
	req := doRaw(t, s.Handler(), rpcCall(1, "server/discover", nil), map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}, "127.0.0.1:1")
	requireRPCError(t, req, http.StatusBadRequest, "unsupported_protocol_version")
}

func TestProtocolVersionMismatch(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "server/discover", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: "2025-11-25",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusBadRequest, "unsupported_protocol_version")
}

func TestPinnedProtocolDiscover(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("missing initialize/discover result")
	}
	if ir.ProtocolVersion != ProtocolVersion {
		t.Fatalf("negotiated %q want %s", ir.ProtocolVersion, ProtocolVersion)
	}
	if buildinfo.Current().Protocols.MCP != ProtocolVersion {
		t.Fatalf("buildinfo MCP=%q", buildinfo.Current().Protocols.MCP)
	}
}

func TestDiscoverAdvertisesOnlyPinnedVersion(t *testing.T) {
	s, _ := newTestServer(t)
	body := rpcCall(1, "server/discover", map[string]any{
		"_meta": map[string]any{
			"io.modelcontextprotocol/protocolVersion": ProtocolVersion,
			"io.modelcontextprotocol/clientInfo": map[string]any{
				"name": "labdns-test", "version": "dev",
			},
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		},
	})
	rec := doRaw(t, s.Handler(), body, map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		"Mcp-Method":          "server/discover",
	}, "127.0.0.1:1")
	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Fatalf("discover status=%d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeRPC(t, rec)
	result, _ := m["result"].(map[string]any)
	if result == nil {
		t.Fatalf("discover missing result: %s", rec.Body.String())
	}
	raw, _ := result["supportedVersions"].([]any)
	if len(raw) != 1 || raw[0] != ProtocolVersion {
		t.Fatalf("supportedVersions=%v want [%s]", raw, ProtocolVersion)
	}
}

func TestStreamableHTTPOnlyPOST(t *testing.T) {
	s, _ := newTestServer(t)
	req := doRawMethod(t, s.Handler(), http.MethodGet, DefaultPath, "", map[string]string{
		"Accept":              "text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}, "127.0.0.1:1")
	if req.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d want 405 body=%s", req.Code, req.Body.String())
	}
}

func TestClosedRejectsRequests(t *testing.T) {
	s, _ := newTestServer(t)
	s.Close()
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}, "127.0.0.1:1")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatelessNoSessionHeader(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	_ = callTool(t, cs, "dns_version_get", map[string]any{})
	_ = callTool(t, cs, "dns_version_get", map[string]any{})
	// Two independent calls on a stateless transport must not require session history.
}

func doRaw(t *testing.T, h http.Handler, body string, hdr map[string]string, remote string) *httptest.ResponseRecorder {
	t.Helper()
	return doRawMethod(t, h, http.MethodPost, DefaultPath, body, hdr, remote)
}

func doRawMethod(t *testing.T, h http.Handler, method, path, body string, hdr map[string]string, remote string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = remote
	for k, v := range hdr {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
