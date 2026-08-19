package rest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/capabilities"
)

func TestRenderOpenAPICoversRegistry(t *testing.T) {
	raw, err := RenderOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != OpenAPIVersion {
		t.Fatalf("openapi=%v", doc["openapi"])
	}
	paths, _ := doc["paths"].(map[string]any)
	if len(paths) == 0 {
		t.Fatal("no paths")
	}
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			item, ok := paths[b.Path].(map[string]any)
			if !ok {
				t.Errorf("missing path %s", b.Path)
				continue
			}
			if item[strings.ToLower(b.Method)] == nil {
				t.Errorf("missing %s %s", b.Method, b.Path)
			}
		}
	}
	comps, _ := doc["components"].(map[string]any)
	schemas, _ := comps["schemas"].(map[string]any)
	if schemas["Spec"] == nil || schemas["Problem"] == nil || schemas["State"] == nil {
		t.Fatalf("missing Spec/Problem/State: %v", keysOf(schemas))
	}
	if schemas["Session"] == nil || schemas["SessionActor"] == nil {
		t.Fatal("missing Session schemas")
	}
	sec, _ := doc["security"].([]any)
	if len(sec) != 2 {
		t.Fatalf("global security=%v", sec)
	}
	schemes, _ := comps["securitySchemes"].(map[string]any)
	if schemes["cookieAuth"] == nil || schemes["bearerAuth"] == nil {
		t.Fatalf("securitySchemes=%v", keysOf(schemes))
	}
	sessionPath, _ := paths["/v1/session"].(map[string]any)
	if sessionPath["post"] == nil || sessionPath["get"] == nil || sessionPath["delete"] == nil {
		t.Fatalf("session ops=%v", sessionPath)
	}
	postSess, _ := sessionPath["post"].(map[string]any)
	if postSess["security"] == nil {
		t.Fatal("POST /v1/session should advertise optional security")
	}
	getSess, _ := sessionPath["get"].(map[string]any)
	getSec, _ := getSess["security"].([]any)
	if len(getSec) != 1 {
		t.Fatalf("GET /v1/session security=%v want cookie-only", getSec)
	}
	sec0, _ := getSec[0].(map[string]any)
	if _, ok := sec0["cookieAuth"]; !ok || len(sec0) != 1 {
		t.Fatalf("GET /v1/session security=%v want cookieAuth only", getSec)
	}
	desc, _ := getSess["description"].(string)
	if !strings.Contains(desc, "live labdns_session cookie") {
		t.Fatalf("GET /v1/session description=%q", desc)
	}
	applyPath, _ := paths["/v1/changes:apply"].(map[string]any)
	postApply, _ := applyPath["post"].(map[string]any)
	if !hasHeaderParam(postApply, "X-LabDNS-CSRF") {
		t.Fatal("mutating ops need X-LabDNS-CSRF")
	}
	getState, _ := paths["/v1/state"].(map[string]any)["get"].(map[string]any)
	if hasHeaderParam(getState, "X-LabDNS-CSRF") {
		t.Fatal("GET must not require CSRF")
	}
	if paths["/"] == nil {
		t.Fatal("missing GET / UI assets path")
	}
	export, _ := paths["/v1/state:export"].(map[string]any)
	getExp, _ := export["get"].(map[string]any)
	params, _ := getExp["parameters"].([]any)
	foundDefault := false
	for _, p := range params {
		pm, _ := p.(map[string]any)
		if pm["name"] != "format" {
			continue
		}
		sch, _ := pm["schema"].(map[string]any)
		if sch["default"] == "yaml" {
			foundDefault = true
		}
	}
	if !foundDefault {
		t.Fatal("export format should default to yaml")
	}
	resp200, _ := getExp["responses"].(map[string]any)["200"].(map[string]any)
	content, _ := resp200["content"].(map[string]any)
	if content["application/yaml"] == nil {
		t.Fatal("export 200 missing application/yaml")
	}
	audit, _ := paths["/v1/audit"].(map[string]any)
	getAudit, _ := audit["get"].(map[string]any)
	auditParams, _ := getAudit["parameters"].([]any)
	for _, p := range auditParams {
		pm, _ := p.(map[string]any)
		if pm["name"] == "cursor" {
			t.Fatal("audit must not advertise cursor until the ring supports it")
		}
	}
}

func TestGeneratedOpenAPIMatchesRender(t *testing.T) {
	want, err := RenderOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(OpenAPIRelPath)))
	if err != nil {
		t.Fatalf("read generated OpenAPI (run make generate): %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is stale; run make generate", OpenAPIRelPath)
	}
}

func hasHeaderParam(op map[string]any, name string) bool {
	params, _ := op["parameters"].([]any)
	for _, p := range params {
		pm, _ := p.(map[string]any)
		if pm["name"] == name && pm["in"] == "header" {
			return true
		}
	}
	return false
}

func keysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
