package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Safe prompts from docs/07. They only name existing tools; they are not capabilities.
var promptCatalog = []struct {
	Name        string
	Title       string
	Description string
	Args        []*sdk.PromptArgument
	Body        func(args map[string]string) string
}{
	{
		Name:        "plan_dns_override",
		Title:       "Plan a DNS override",
		Description: "Guide an agent through reading state, planning a typed override, and applying only after review. Uses existing tools only.",
		Args: []*sdk.PromptArgument{
			{Name: "name", Description: "FQDN to override", Required: true},
			{Name: "type", Description: "RR type (A, AAAA, CNAME, …)", Required: true},
			{Name: "value", Description: "Record value (address or target)", Required: true},
		},
		Body: func(args map[string]string) string {
			return fmt.Sprintf(`Plan a DNS override for %s %s → %s.
1. Call dns_state_get and note runtimeRevision.
2. Call dns_change_plan with expectedRevision and a typed add/update operation. Do not apply yet.
3. Review the normalized diff, impact.names, wildcardCoverage, and suggestedProbes.
4. Obtain human approval when required.
5. Call dns_change_apply with the same expectedRevision and an idempotencyKey.
6. Call dns_resolve and dns_explain_resolution as probes.
Do not invent tools. Do not skip the plan step.`, args["name"], args["type"], args["value"])
		},
	},
	{
		Name:        "diagnose_resolution",
		Title:       "Diagnose why a name resolved a certain way",
		Description: "Explain a live answer using resolve/explain and the DNS semantics document. Read-only.",
		Args: []*sdk.PromptArgument{
			{Name: "name", Description: "QNAME to explain", Required: true},
			{Name: "type", Description: "QTYPE", Required: true},
		},
		Body: func(args map[string]string) string {
			return fmt.Sprintf(`Diagnose why %s %s resolved the way it did.
1. Call dns_explain_resolution with the name and type (and optional clientContext).
2. If needed, call dns_resolve with applyChaos=false (management default).
3. Read labdns://docs/dns-semantics or dns_docs_get id=dns-semantics for flag and wildcard rules.
4. If forwarding or chaos may be involved, call dns_upstreams_status and dns_chaos_simulate (simulation never sleeps).
Do not mutate state.`, args["name"], args["type"])
		},
	},
	{
		Name:        "design_chaos_experiment",
		Title:       "Design a bounded chaos experiment",
		Description: "Design a chaos policy using simulation and safety docs. Activation is a separate high-impact tool.",
		Args: []*sdk.PromptArgument{
			{Name: "name", Description: "Target QNAME", Required: true},
			{Name: "type", Description: "QTYPE", Required: true},
		},
		Body: func(args map[string]string) string {
			return fmt.Sprintf(`Design a bounded chaos experiment for %s %s.
1. Call dns_chaos_status and dns_docs_get id=chaos-safety.
2. Call dns_chaos_simulate with the query; do not activate anything yet.
3. Check safety class, expiry, protected names, and global delay/concurrency caps in the impact summary of a dns_change_plan.
4. Only after review, dns_chaos_activate (high-impact) with expectedRevision.
Never request malformed-wire generation. Simulation must not sleep or send packets.`, args["name"], args["type"])
		},
	},
	{
		Name:        "convert_runtime_drift",
		Title:       "Convert runtime drift into a deployment-repository change",
		Description: "Turn in-memory drift into GitOps operations via export. Does not write the bootstrap file.",
		Args:        []*sdk.PromptArgument{},
		Body: func(map[string]string) string {
			return `Convert runtime drift into a deployment-repository change.
1. Call dns_status_get and inspect revisions.drifted.
2. Call dns_state_export (format=yaml) and use bootstrapToRuntime as the typed operations.
3. Do not call dns_state_reset unless an operator explicitly wants to discard runtime drift.
4. The service never writes the bootstrap file; persist the export in the deployment repository.
Use only existing tools.`
		},
	},
}

func (s *Server) registerPrompts() {
	for _, p := range promptCatalog {
		p := p
		s.sdk.AddPrompt(&sdk.Prompt{
			Name:        p.Name,
			Title:       p.Title,
			Description: p.Description,
			Arguments:   p.Args,
		}, func(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
			args := map[string]string{}
			if req != nil && req.Params != nil && req.Params.Arguments != nil {
				args = req.Params.Arguments
			}
			return &sdk.GetPromptResult{
				Description: p.Description,
				Messages: []*sdk.PromptMessage{{
					Role:    "user",
					Content: &sdk.TextContent{Text: p.Body(args)},
				}},
			}, nil
		})
	}
}

// PromptNames is the frozen pack-07 set. Tests lock this list.
func PromptNames() []string {
	out := make([]string, len(promptCatalog))
	for i, p := range promptCatalog {
		out[i] = p.Name
	}
	return out
}
