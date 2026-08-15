package observability

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogLabelPolicy(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range Metrics() {
		if m.Name == "" || m.Kind == "" || m.Help == "" {
			t.Fatalf("incomplete metric %+v", m)
		}
		if seen[m.Name] {
			t.Fatalf("duplicate metric %s", m.Name)
		}
		seen[m.Name] = true
		if !strings.HasPrefix(m.Name, "labdns_") {
			t.Fatalf("metric %s missing labdns_ prefix", m.Name)
		}
		for _, l := range m.Labels {
			if ForbiddenLabel(l) {
				t.Fatalf("%s declares forbidden label %q", m.Name, l)
			}
			if !AllowedLabel(l) {
				t.Fatalf("%s declares undeclared label %q", m.Name, l)
			}
		}
		lower := strings.ToLower(m.Name + " " + strings.Join(m.Labels, " "))
		if strings.Contains(lower, "qname") || strings.Contains(lower, "client_ip") {
			t.Fatalf("catalog row mentions qname/client_ip: %s", m.Name)
		}
	}
	if _, ok := LookupMetric(MetricTelemetryDropped); !ok {
		t.Fatal("missing drop metric")
	}
}

func TestCatalogMatchesGeneratedArtifact(t *testing.T) {
	got, err := RenderCatalog()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repoRoot(t), CatalogRelPath)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run make generate)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run make generate", CatalogRelPath)
	}
	var doc Document
	if err := json.Unmarshal(want, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.ID != CatalogID {
		t.Fatalf("id=%s", doc.ID)
	}
	if len(doc.Metrics) != len(Metrics()) || len(doc.Events) != len(Events()) {
		t.Fatalf("artifact metrics=%d events=%d", len(doc.Metrics), len(doc.Events))
	}
}

func TestEventsHaveStableFields(t *testing.T) {
	if len(Events()) == 0 {
		t.Fatal("no events")
	}
	for _, e := range Events() {
		if e.Name == "" {
			t.Fatal("empty event name")
		}
		joined := strings.Join(e.Fields, ",")
		if !strings.Contains(joined, "event") || !strings.Contains(joined, "request_id") {
			t.Fatalf("event %s missing required fields", e.Name)
		}
	}
}

func TestForbiddenAndAllowed(t *testing.T) {
	for _, k := range []string{"qname", "QNAME", "client_ip", "actor_id", "idempotency_key", "error"} {
		if !ForbiddenLabel(k) {
			t.Fatalf("%s should be forbidden", k)
		}
	}
	for _, k := range []string{"zone_id", "policy_id", "transport", "rcode", "result"} {
		if !AllowedLabel(k) {
			t.Fatalf("%s should be allowed", k)
		}
		if ForbiddenLabel(k) {
			t.Fatalf("%s must not also be forbidden", k)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
