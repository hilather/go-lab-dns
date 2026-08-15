package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/hilather/go-lab-dns/internal/control/rest"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestStructuredDTOMatchesREST(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	rs, err := rest.New(rest.Config{Service: svc})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := New(Config{Service: svc})
	if err != nil {
		t.Fatal(err)
	}
	ts := startHTTP(t, ms)
	cs := connectClient(t, ts)

	statusREST := restGET(t, rs.Handler(), "/v1/status")
	statusMCP := structuredMap(t, callTool(t, cs, "dns_status_get", map[string]any{}))
	assertCamelCase(t, "status", statusMCP)
	assertEqualPath(t, "status.cache.enabled", nest(statusREST, "cache", "enabled"), nest(statusMCP, "cache", "enabled"))
	assertEqualPath(t, "status.chaos.emergencyDisabled", nest(statusREST, "chaos", "emergencyDisabled"), nest(statusMCP, "chaos", "emergencyDisabled"))

	cacheREST := restGET(t, rs.Handler(), "/v1/cache/status")
	cacheMCP := structuredMap(t, callTool(t, cs, "dns_cache_status", map[string]any{}))
	assertCamelCase(t, "cache", cacheMCP)
	assertEqualPath(t, "cache.enabled", cacheREST["enabled"], cacheMCP["enabled"])
	assertEqualPath(t, "cache.maxEntries", cacheREST["maxEntries"], cacheMCP["maxEntries"])

	upsREST := restGET(t, rs.Handler(), "/v1/upstreams/status")
	upsMCP := structuredMap(t, callTool(t, cs, "dns_upstreams_status", map[string]any{}))
	assertCamelCase(t, "upstreams", upsMCP)
	if firstREST, ok := firstObj(upsREST["upstreams"]); ok {
		if firstMCP, ok := firstObj(upsMCP["upstreams"]); ok {
			assertEqualPath(t, "upstream.poolId", firstREST["poolId"], firstMCP["poolId"])
			if firstMCP["PoolID"] != nil {
				t.Fatal("upstream leaked PascalCase PoolID")
			}
		}
	}

	chaosREST := restGET(t, rs.Handler(), "/v1/chaos/status")
	chaosMCP := structuredMap(t, callTool(t, cs, "dns_chaos_status", map[string]any{}))
	assertCamelCase(t, "chaos", chaosMCP)
	assertEqualPath(t, "chaos.enabled", chaosREST["enabled"], chaosMCP["enabled"])

	simBody := `{"name":"foo.tools.lab.example.net.","type":"A","clientContext":{"clientGroup":"test-devices"},"nonce":"sim"}`
	simREST := restPOST(t, rs.Handler(), "/v1/chaos:simulate", simBody)
	simMCP := structuredMap(t, callTool(t, cs, "dns_chaos_simulate", map[string]any{
		"name": "foo.tools.lab.example.net.", "type": "A",
		"clientContext": map[string]any{"clientGroup": "test-devices"},
		"nonce":         "sim",
	}))
	assertCamelCase(t, "simulate", simMCP)
	assertEqualPath(t, "simulate.algorithm", simREST["algorithm"], simMCP["algorithm"])

	rev, _ := structuredMap(t, callTool(t, cs, "dns_state_get", map[string]any{}))["runtimeRevision"].(string)
	op := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab-zone"},
		Value:  json.RawMessage(`{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}`),
	}
	planArgs := map[string]any{"expectedRevision": rev, "reason": "add www", "operations": []model.Operation{op}}
	planREST := restPOST(t, rs.Handler(), "/v1/changes:plan", compactJSON(t, planArgs))
	planMCP := structuredMap(t, callTool(t, cs, "dns_change_plan", planArgs))
	assertCamelCase(t, "plan", planMCP)
	assertEqualPath(t, "plan.impact.wildcardCoverage", nest(planREST, "impact", "wildcardCoverage"), nest(planMCP, "impact", "wildcardCoverage"))
	if nest(planMCP, "impact", "WildcardCoverage") != nil {
		t.Fatal("plan impact leaked PascalCase WildcardCoverage")
	}

	auditREST := restGET(t, rs.Handler(), "/v1/audit")
	auditMCP := structuredMap(t, callTool(t, cs, "dns_audit_query", map[string]any{}))
	assertCamelCase(t, "audit", auditMCP)
	if firstREST, ok := firstObj(auditREST["events"]); ok {
		if firstMCP, ok := firstObj(auditMCP["events"]); ok {
			assertEqualPath(t, "audit.id", firstREST["id"], firstMCP["id"])
		}
	}
}

func restGET(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	return restDo(t, h, http.MethodGet, path, "")
}

func restPOST(t *testing.T, h http.Handler, path, body string) map[string]any {
	t.Helper()
	return restDo(t, h, http.MethodPost, path, body)
}

func restDo(t *testing.T, h http.Handler, method, path, body string) map[string]any {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = "127.0.0.1:54321"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s status=%d body=%s", method, path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("rest json %s: %v body=%s", path, err, rec.Body.String())
	}
	return out
}

func nest(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[k]
	}
	return cur
}

func firstObj(v any) (map[string]any, bool) {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return nil, false
	}
	obj, ok := arr[0].(map[string]any)
	return obj, ok
}

func assertEqualPath(t *testing.T, path string, restv, mcpv any) {
	t.Helper()
	rs, _ := json.Marshal(restv)
	ms, _ := json.Marshal(mcpv)
	if string(rs) != string(ms) {
		t.Errorf("%s REST=%s MCP=%s", path, rs, ms)
	}
}

func assertCamelCase(t *testing.T, label string, v any) {
	t.Helper()
	walkKeys(v, func(k string) {
		if k == "" {
			return
		}
		r := rune(k[0])
		if unicode.IsUpper(r) {
			t.Errorf("%s leaked PascalCase key %q", label, k)
		}
	})
}

func walkKeys(v any, fn func(string)) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			fn(k)
			walkKeys(child, fn)
		}
	case []any:
		for _, child := range x {
			walkKeys(child, fn)
		}
	}
}
