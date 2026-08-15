package mcp

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/capabilities"
)

func TestRenderManifestContainsPin(t *testing.T) {
	raw, err := RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, ProtocolVersion) {
		t.Fatal("missing protocol pin")
	}
	if !strings.Contains(body, SDKModule) {
		t.Fatal("missing SDK module")
	}
	if !strings.Contains(body, ManifestGeneratedBy) {
		t.Fatal("missing generated-by")
	}
	for _, name := range capabilities.Tools() {
		if !strings.Contains(body, name) {
			t.Errorf("manifest missing tool %s", name)
		}
	}
	for _, uri := range capabilities.Resources() {
		if !strings.Contains(body, uri) {
			t.Errorf("manifest missing resource %s", uri)
		}
	}
	if strings.Contains(body, `"dns_health`) {
		t.Fatal("health tools must not appear")
	}
}
