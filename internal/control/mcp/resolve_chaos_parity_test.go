package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/control/rest"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestManagementChaosResolveExplainRESTMCPParity(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	policy := svc.Store().Load().Canonical.Spec.Chaos.Policies[0]
	policy.Enabled = true
	policy.Outcomes[0].Actions = append(policy.Outcomes[0].Actions, model.ChaosAction{Type: model.ActionTTL, Phase: model.PhaseBeforeResponse, Value: "zero"})
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(t.Context(), auth.Actor{Role: auth.RoleAdministrator}, app.ChangeIn{ExpectedRevision: svc.Store().Load().Revision, Operations: []model.Operation{{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetChaosPolicy, ID: string(policy.ID)}, Value: raw}}}); err != nil {
		t.Fatal(err)
	}
	authn := devLoopbackAuth(t)
	rs, err := rest.New(rest.Config{Service: svc, Auth: authn})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := New(Config{Service: svc, Auth: authn})
	if err != nil {
		t.Fatal(err)
	}
	client := connectClient(t, startHTTP(t, ms))
	for _, tc := range []struct{ path, tool string }{{"/v1/resolve", "dns_resolve"}, {"/v1/resolve:explain", "dns_explain_resolution"}} {
		arguments := map[string]any{"name": "x.tools.lab.example.net.", "type": "A", "clientContext": map[string]any{"clientGroup": "test-devices", "transport": "udp"}, "options": map[string]any{"applyChaos": true}}
		body, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(body)))
		request.RemoteAddr = "127.0.0.1:1"
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		rs.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("REST: %d %s", recorder.Code, recorder.Body.String())
		}
		var restOutput map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &restOutput); err != nil {
			t.Fatal(err)
		}
		mcpOutput := structuredMap(t, callTool(t, client, tc.tool, arguments))
		if !reflect.DeepEqual(restOutput, mcpOutput) {
			t.Fatalf("REST/MCP chaos parity: rest=%+v mcp=%+v", restOutput, mcpOutput)
		}
		result := restOutput["result"].(map[string]any)
		explanation := result["explanation"].(map[string]any)
		if len(explanation["chaosDecisions"].([]any)) == 0 || len(explanation["baseAnswers"].([]any)) == 0 {
			t.Fatalf("modeled decision/base absent: %+v", explanation)
		}
		if result["answers"].([]any)[0].(map[string]any)["ttl"] != "0s" {
			t.Fatalf("final modeled answer unchanged: %+v", result)
		}
	}
}
