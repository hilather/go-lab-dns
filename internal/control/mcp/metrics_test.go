package mcp

import (
	"github.com/hilather/go-lab-dns/internal/observability"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestCapabilityMetricsForToolsAndResources(t *testing.T) {
	registry := observability.NewRegistry()
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	s, err := New(Config{Service: svc, Auth: devLoopbackAuth(t), Metrics: registry})
	if err != nil {
		t.Fatal(err)
	}
	cs := connectClient(t, startHTTP(t, s))
	if _, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "dns_version_get", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "labdns://status"}); err != nil {
		t.Fatal(err)
	}
	calls := float64(0)
	for _, sample := range registry.Snapshot() {
		if sample.Name == observability.MetricCapabilityCalls && sample.Labels["transport"] == "mcp" && sample.Labels["result"] == "ok" {
			calls += sample.Value
		}
	}
	if calls != 2 {
		t.Fatalf("MCP capability calls=%v want 2", calls)
	}
}
