package mcp

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
)

func TestMCPRoleMatrix(t *testing.T) {
	pol, err := auth.NewPolicy(auth.PolicyConfig{Tokens: []auth.Token{
		{Token: "viewer", ID: "v", Role: auth.RoleViewer},
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
	hdr := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Bearer viewer",
	}
	body := rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_version_get",
		"arguments": map[string]any{},
	})
	ok := doRaw(t, s.Handler(), body, hdr, "192.0.2.10:9")
	if ok.Code == 401 || ok.Code == 403 {
		t.Fatalf("viewer version denied: %s", ok.Body.String())
	}
	reset := rpcCall(1, "tools/call", map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"name":      "dns_state_reset",
		"arguments": map[string]any{"reason": "no"},
	})
	denied := doRaw(t, s.Handler(), reset, hdr, "192.0.2.10:9")
	if denied.Code == 401 {
		t.Fatalf("auth failed: %s", denied.Body.String())
	}
	// Tool errors stay 200 with isError; either 403 RPC or tool error is fine.
	if denied.Code == 200 && !containsForbidden(denied.Body.String()) {
		t.Fatalf("viewer reset not forbidden: %s", denied.Body.String())
	}
}

func containsForbidden(s string) bool {
	return len(s) > 0 && (containsWord(s, "forbidden") || containsWord(s, "missing scope"))
}

func containsWord(s, w string) bool {
	return len(s) >= len(w) && (s == w || len(s) > 0 && (indexOf(s, w) >= 0))
}

func indexOf(s, w string) int {
	for i := 0; i+len(w) <= len(s); i++ {
		if s[i:i+len(w)] == w {
			return i
		}
	}
	return -1
}
