package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/control/rest"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestDurationStringAuthorizationRESTMCPParity(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	policy, err := auth.NewPolicy(auth.PolicyConfig{Tokens: []auth.Token{
		{Token: "editor", Role: auth.RoleDNSEditor}, {Token: "designer", Role: auth.RoleChaosDesigner},
	}})
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rest.New(rest.Config{Service: svc, Auth: policy, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := New(Config{Service: svc, Auth: policy, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token string
		op          model.Operation
	}{
		{"record", "editor", model.Operation{Op: model.OpAdd, Target: model.Target{Kind: model.TargetRecord, ID: "duration-a", ZoneID: "lab-zone"}, Value: json.RawMessage(`{"id":"duration-a","owner":"ordinary","type":"A","ttl":"30s","values":["192.0.2.1"]}`)}},
		{"policy", "designer", model.Operation{Op: model.OpAdd, Target: model.Target{Kind: model.TargetChaosPolicy, ID: "duration-policy"}, Value: json.RawMessage(`{"id":"duration-policy","owner":"lab","reason":"test","enabled":false,"safetyClass":"low","scope":{"recordIds":["tools-wildcard-a"]},"selector":{"mode":"deterministic","seed":"test","probability":1,"timeBucket":"1s"},"outcomes":[{"id":"o","weight":1,"actions":[{"type":"delay","phase":"before-response","distribution":"fixed","duration":"100ms"}]}]}`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{"expectedRevision": svc.Store().Load().Revision, "operations": []model.Operation{tc.op}}
			body, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/changes:plan", strings.NewReader(string(body)))
			req.RemoteAddr = "192.0.2.10:9"
			req.Header.Set("Authorization", "Bearer "+tc.token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			rs.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("REST: %d %s", rec.Code, rec.Body.String())
			}
			rpc := rpcCall(1, "tools/call", map[string]any{"name": "dns_change_plan", "arguments": args, "_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    ProtocolVersion,
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
				"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "duration-test", "version": "dev"},
			}})
			result := doRaw(t, ms.Handler(), rpc, map[string]string{"Mcp-Method": "tools/call", "Mcp-Name": "dns_change_plan", "Content-Type": "application/json", "Accept": "application/json, text/event-stream", headerProtocolVersion: ProtocolVersion, headerAuthorization: "Bearer " + tc.token}, "192.0.2.10:9")
			if !strings.Contains(result.Body.String(), `"candidateRevision"`) || strings.Contains(result.Body.String(), `"isError":true`) {
				t.Fatalf("MCP: %d %s", result.Code, result.Body.String())
			}
		})
	}
}
