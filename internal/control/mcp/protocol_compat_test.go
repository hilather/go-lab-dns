package mcp

import (
	"net/http"
	"testing"
)

func newCompatServer(t *testing.T) *Server {
	t.Helper()
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowAnyProtocolVersion: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Regression for gateway interop: clients on earlier SDK generations (e.g.
// mark3labs/mcp-go) send an old-style initialize without the 2026-07-28
// header or _meta envelope. With AllowAnyProtocolVersion the SDK negotiates
// the client's version instead of rejecting it.
func TestCompatNegotiatesOlderInitialize(t *testing.T) {
	s := newCompatServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"legacy-gateway","version":"0"}}}`
	rec := doRawMethod(t, s.Handler(), http.MethodPost, DefaultPath, body, map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}, "127.0.0.1:1")
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeRPC(t, rec)
	result, _ := m["result"].(map[string]any)
	if result == nil {
		t.Fatalf("missing result: %s", rec.Body.String())
	}
	if got, _ := result["protocolVersion"].(string); got != "2025-03-26" {
		t.Fatalf("negotiated %q, want client's 2025-03-26", got)
	}
}

// Legacy requests carry neither the MCP-Protocol-Version header nor the v2
// _meta envelope; the compat server serves them. (When _meta is present the
// SDK still enforces header/_meta consistency — compat only relaxes the pin.)
func TestCompatLegacyToolsList(t *testing.T) {
	s := newCompatServer(t)
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	rec := doRawMethod(t, s.Handler(), http.MethodPost, DefaultPath, body, map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}, "127.0.0.1:1")
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy tools/list status=%d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeRPC(t, rec)
	result, _ := m["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("no tools listed: %s", rec.Body.String())
	}
}

// The pin stays the default: TestProtocolVersionRequired and
// TestProtocolVersionMismatch in protocol_test.go cover the strict path.
func TestCompatOffByDefault(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "server/discover", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: "2025-11-25",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusBadRequest, "unsupported_protocol_version")
}
