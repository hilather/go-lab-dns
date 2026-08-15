package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsRegisteredFromRegistry(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	seen := map[string]bool{}
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		seen[tool.Name] = true
		if tool.InputSchema == nil {
			t.Errorf("%s missing input schema", tool.Name)
		}
	}
	for _, name := range capabilities.Tools() {
		if !seen[name] {
			t.Errorf("missing tool %s", name)
		}
	}
	if seen["health.live"] || seen["dns_health_live"] {
		t.Fatal("health live must not be a tool")
	}
}

func TestResourcesRegisteredFromRegistry(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	seen := map[string]bool{}
	for r, err := range cs.Resources(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		seen[r.URI] = true
	}
	for tmpl, err := range cs.ResourceTemplates(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		seen[tmpl.URITemplate] = true
	}
	for _, uri := range capabilities.Resources() {
		if !seen[uri] {
			t.Errorf("missing resource %s", uri)
		}
	}
}

func TestContractReads(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	ver := structuredMap(t, callTool(t, cs, "dns_version_get", map[string]any{}))
	if ver["protocols"] == nil {
		t.Fatalf("version=%v", ver)
	}
	caps := structuredMap(t, callTool(t, cs, "dns_capabilities_get", map[string]any{}))
	if caps["capabilities"] == nil {
		t.Fatalf("capabilities=%v", caps)
	}
	st := structuredMap(t, callTool(t, cs, "dns_status_get", map[string]any{}))
	if st["revisions"] == nil {
		t.Fatalf("status=%v", st)
	}
	schema := callTool(t, cs, "dns_schema_get", map[string]any{})
	raw, _ := json.Marshal(schema.StructuredContent)
	if !strings.Contains(string(raw), "labdns.dev/v1alpha1") {
		t.Fatalf("schema missing api version: %s", raw)
	}
	docs := structuredMap(t, callTool(t, cs, "dns_docs_get", map[string]any{"id": "dns-semantics"}))
	if docs["markdown"] == nil {
		t.Fatalf("docs=%v", docs)
	}
	_ = structuredMap(t, callTool(t, cs, "dns_docs_get", map[string]any{"id": "chaos-safety"}))

	state := structuredMap(t, callTool(t, cs, "dns_state_get", map[string]any{}))
	if state["runtimeRevision"] == "" {
		t.Fatalf("state=%v", state)
	}
	zones := structuredMap(t, callTool(t, cs, "dns_zones_list", map[string]any{}))
	if zones["zones"] == nil {
		t.Fatalf("zones=%v", zones)
	}
	zone := structuredMap(t, callTool(t, cs, "dns_zone_get", map[string]any{"zoneId": "lab-zone"}))
	if zone["id"] != "lab-zone" {
		t.Fatalf("zone=%v", zone)
	}
	missing := callToolExpectError(t, cs, "dns_zone_get", map[string]any{"zoneId": "nope"})
	if domainCode(t, missing) != "not_found" {
		t.Fatalf("missing zone error=%v", missing)
	}
	recs := structuredMap(t, callTool(t, cs, "dns_records_list", map[string]any{"zoneId": "lab-zone", "limit": 1}))
	if recs["records"] == nil {
		t.Fatalf("records=%v", recs)
	}
	rec := structuredMap(t, callTool(t, cs, "dns_record_get", map[string]any{"zoneId": "lab-zone", "recordId": "ns1-a"}))
	if rec["id"] != "ns1-a" {
		t.Fatalf("record=%v", rec)
	}
	resolve := structuredMap(t, callTool(t, cs, "dns_resolve", map[string]any{"name": "ns1.lab.example.net", "type": "A"}))
	if resolve["result"] == nil {
		t.Fatalf("resolve=%v", resolve)
	}
	explain := structuredMap(t, callTool(t, cs, "dns_explain_resolution", map[string]any{
		"name": "ns1.lab.example.net.", "type": "A",
		"clientContext": map[string]any{"client": "127.0.0.1", "transport": "udp"},
	}))
	if explain["result"] == nil {
		t.Fatalf("explain=%v", explain)
	}

	stateRes, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "labdns://state"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stateRes.Contents) == 0 || !strings.Contains(stateRes.Contents[0].Text, "runtimeRevision") {
		t.Fatalf("resource state=%+v", stateRes)
	}
}

func TestContractMutations(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	state := structuredMap(t, callTool(t, cs, "dns_state_get", map[string]any{}))
	rev, _ := state["runtimeRevision"].(string)
	op := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab-zone"},
		Value:  json.RawMessage(`{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}`),
	}
	args := map[string]any{
		"expectedRevision": rev,
		"reason":           "add www",
		"operations":       []model.Operation{op},
	}
	val := structuredMap(t, callTool(t, cs, "dns_state_validate", map[string]any{
		"operations": []model.Operation{op},
	}))
	if val["candidateRevision"] == nil && val["previousRevision"] == nil {
		t.Fatalf("validate=%v", val)
	}
	plan := structuredMap(t, callTool(t, cs, "dns_change_plan", args))
	if plan["candidateRevision"] == rev {
		t.Fatal("plan did not change revision")
	}
	bad := callToolExpectError(t, cs, "dns_change_apply", map[string]any{
		"expectedRevision": "sha256:deadbeef",
		"operations":       []model.Operation{op},
	})
	if domainCode(t, bad) != "revision_conflict" {
		t.Fatalf("apply conflict=%v", bad)
	}
	apply := structuredMap(t, callTool(t, cs, "dns_change_apply", args))
	if apply["applied"] != true {
		t.Fatalf("apply=%v", apply)
	}
	exp := structuredMap(t, callTool(t, cs, "dns_state_export", map[string]any{"format": "yaml"}))
	body, _ := exp["body"].(string)
	if !strings.Contains(body, "www-a") {
		t.Fatalf("export missing www-a: %v", exp)
	}
	reset := structuredMap(t, callTool(t, cs, "dns_state_reset", map[string]any{"reason": "test"}))
	if reset["applied"] != true {
		t.Fatalf("reset=%v", reset)
	}
	missing := callToolExpectError(t, cs, "dns_record_get", map[string]any{"zoneId": "lab-zone", "recordId": "www-a"})
	if domainCode(t, missing) != "not_found" {
		t.Fatalf("after reset: %v", missing)
	}
}

func TestIdempotentApplySameKey(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	rev, _ := structuredMap(t, callTool(t, cs, "dns_state_get", map[string]any{}))["runtimeRevision"].(string)
	args := map[string]any{
		"expectedRevision": rev,
		"idempotencyKey":   "k1",
		"reason":           "add",
		"operations": []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab-zone"},
			Value:  json.RawMessage(`{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}`),
		}},
	}
	a1 := structuredMap(t, callTool(t, cs, "dns_change_apply", args))
	a2 := structuredMap(t, callTool(t, cs, "dns_change_apply", args))
	if a1["candidateRevision"] != a2["candidateRevision"] {
		t.Fatal("idempotent apply returned different revisions")
	}
	conflict := callToolExpectError(t, cs, "dns_change_apply", map[string]any{
		"expectedRevision": rev,
		"idempotencyKey":   "k1",
		"reason":           "other",
		"operations":       []any{},
	})
	if domainCode(t, conflict) != "idempotency_conflict" {
		t.Fatalf("idempotency: %v", conflict)
	}
}

func TestContractForwardingCacheAudit(t *testing.T) {
	s, _ := newTestServerFixture(t, "pack-sample.yaml")
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	_ = structuredMap(t, callTool(t, cs, "dns_forwarding_policies_list", map[string]any{}))
	_ = structuredMap(t, callTool(t, cs, "dns_upstream_pools_list", map[string]any{}))
	_ = structuredMap(t, callTool(t, cs, "dns_upstreams_status", map[string]any{}))
	_ = structuredMap(t, callTool(t, cs, "dns_cache_status", map[string]any{}))
	flush := structuredMap(t, callTool(t, cs, "dns_cache_flush", map[string]any{"all": true}))
	if flush["ok"] != true {
		t.Fatalf("flush=%v", flush)
	}
	_ = structuredMap(t, callTool(t, cs, "dns_audit_query", map[string]any{}))
	missing := callToolExpectError(t, cs, "dns_audit_get", map[string]any{"id": "nope"})
	if domainCode(t, missing) != "not_found" {
		t.Fatalf("audit get: %v", missing)
	}
}

func callToolExpectError(t *testing.T, cs *sdk.ClientSession, name string, args any) error {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return err
	}
	if res != nil && res.IsError {
		raw, _ := json.Marshal(res.StructuredContent)
		return &toolDomainError{raw: raw, text: firstText(res)}
	}
	t.Fatalf("CallTool %s: want error", name)
	return nil
}

type toolDomainError struct {
	raw  []byte
	text string
}

func (e *toolDomainError) Error() string { return e.text }

func firstText(res *sdk.CallToolResult) string {
	if res == nil {
		return ""
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			return tc.Text
		}
	}
	return "tool error"
}

func domainCode(t *testing.T, err error) string {
	t.Helper()
	var te *toolDomainError
	if errors.As(err, &te) && len(te.raw) > 0 {
		var payload struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(te.raw, &payload) == nil && payload.Code != "" {
			return payload.Code
		}
	}
	var werr *jsonrpc.Error
	if errors.As(err, &werr) && len(werr.Data) > 0 {
		var payload struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(werr.Data, &payload) == nil && payload.Code != "" {
			return payload.Code
		}
	}
	s := err.Error()
	for _, code := range []string{"not_found", "revision_conflict", "idempotency_conflict", "validation_failed"} {
		if strings.Contains(s, code) {
			return code
		}
	}
	t.Logf("error text=%s", s)
	return s
}
