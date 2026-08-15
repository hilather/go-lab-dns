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

func keysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
