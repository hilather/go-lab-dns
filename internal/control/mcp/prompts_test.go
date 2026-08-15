package mcp

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/capabilities"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	promptToolRE     = regexp.MustCompile(`\bdns_[a-z0-9_]+`)
	promptResourceRE = regexp.MustCompile(`labdns://[a-z0-9_./{}-]+`)
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

	args := map[string]string{"name": "www.lab.example.net.", "type": "A", "value": "10.42.0.80"}
	for _, name := range PromptNames() {
		got, err := cs.GetPrompt(t.Context(), &sdk.GetPromptParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("GetPrompt %s: %v", name, err)
		}
		if len(got.Messages) == 0 {
			t.Fatalf("empty prompt %s", name)
		}
		text := got.Messages[0].Content.(*sdk.TextContent).Text
		if strings.Contains(text, "shell") || strings.Contains(text, "/bin/") {
			t.Fatalf("%s must not introduce new capabilities", name)
		}
		for _, tool := range promptToolRE.FindAllString(text, -1) {
			if len(capabilities.LookupTool(tool)) == 0 {
				t.Errorf("%s names unknown tool %s", name, tool)
			}
		}
		for _, uri := range promptResourceRE.FindAllString(text, -1) {
			if _, ok := capabilities.LookupResource(uri); !ok {
				t.Errorf("%s names unknown resource %s", name, uri)
			}
		}
	}
}
