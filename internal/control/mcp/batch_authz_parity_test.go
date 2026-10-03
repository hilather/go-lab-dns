package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/control/rest"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestBatchHighImpactAuthorizationRESTMCPParity(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	policy, err := auth.NewPolicy(auth.PolicyConfig{Tokens: []auth.Token{{Token: "limited", ID: "limited", Scopes: []string{auth.ScopeChaosWrite, auth.ScopeChaosActivate}}}})
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
	state, err := svc.GetState(t.Context(), auth.Actor{Role: auth.RoleAdministrator})
	if err != nil {
		t.Fatal(err)
	}
	p := state.Canonical.Spec.Chaos.Policies[0]
	p.ID = "batch-high"
	p.SafetyClass = model.SafetyClassHigh
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	in := app.ChangeIn{ExpectedRevision: state.RuntimeRevision, Operations: []model.Operation{
		{Op: model.OpAdd, Target: model.Target{Kind: model.TargetChaosPolicy, ID: string(p.ID)}, Value: raw},
		{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetChaosActivation, ID: string(p.ID)}, Value: json.RawMessage(`{"enabled":true,"expiresAt":"2099-01-01T00:00:00Z"}`)},
	}}
	for _, action := range []string{"plan", "apply", "validate"} {
		t.Run(action, func(t *testing.T) {
			path, tool := "/v1/changes:"+action, "dns_change_"+action
			var input any = in
			if action == "validate" {
				path, tool = "/v1/state:validate", "dns_state_validate"
				input = app.ValidateIn{Operations: in.Operations}
			}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
			req.RemoteAddr = "192.0.2.10:9"
			req.Header.Set("Authorization", "Bearer limited")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			rs.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"code":"forbidden"`) {
				t.Fatalf("REST: %d %s", rec.Code, rec.Body.String())
			}
			arguments := map[string]any{"operations": in.Operations}
			if action != "validate" {
				arguments["expectedRevision"] = in.ExpectedRevision
			}
			rpc := rpcCall(1, "tools/call", map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion, "io.modelcontextprotocol/clientCapabilities": map[string]any{}, "io.modelcontextprotocol/clientInfo": map[string]any{"name": "batch-test", "version": "dev"}}, "name": tool, "arguments": arguments})
			result := doRaw(t, ms.Handler(), rpc, map[string]string{"Mcp-Name": tool, "Mcp-Method": "tools/call", "Content-Type": "application/json", "Accept": "application/json, text/event-stream", headerProtocolVersion: ProtocolVersion, headerAuthorization: "Bearer limited"}, "192.0.2.10:9")
			if !strings.Contains(result.Body.String(), "forbidden") {
				t.Fatalf("MCP: %d %s", result.Code, result.Body.String())
			}
		})
	}
}
