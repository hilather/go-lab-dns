package mcp

import (
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFourSafePrompts(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	want := map[string]bool{
		"plan_dns_override":       false,
		"diagnose_resolution":     false,
		"design_chaos_experiment": false,
		"convert_runtime_drift":   false,
	}
	for p, err := range cs.Prompts(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := want[p.Name]; !ok {
			t.Errorf("unexpected prompt %s", p.Name)
		}
		want[p.Name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing prompt %s", name)
		}
	}

	got, err := cs.GetPrompt(t.Context(), &sdk.GetPromptParams{
		Name:      "plan_dns_override",
		Arguments: map[string]string{"name": "www.lab.example.net.", "type": "A", "value": "10.42.0.80"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) == 0 {
		t.Fatal("empty prompt")
	}
	text := got.Messages[0].Content.(*sdk.TextContent).Text
	for _, tool := range []string{"dns_state_get", "dns_change_plan", "dns_change_apply"} {
		if !strings.Contains(text, tool) {
			t.Errorf("prompt missing tool %s", tool)
		}
	}
	if strings.Contains(text, "shell") || strings.Contains(text, "/bin/") {
		t.Fatal("prompt must not introduce new capabilities")
	}
}
