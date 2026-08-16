// Package releasecontract is the shared list of public surfaces, required
// CI jobs, and release-note headings. Generate, release-diff, and
// checktargets must not drift independently.
package releasecontract

// Surface is one public compatibility file compared between git refs.
type Surface struct {
	// ID is the stable machine name used in reports and --notes accounting.
	ID string
	// Path is relative to the module root, slash-separated.
	Path string
	// Title is the human name that must appear in release notes when the file changes.
	Title string
	// ReviewToken is a case-insensitive substring that accounts for this surface
	// in the "Complete functionality-difference review" checklist.
	ReviewToken string
	// Generated is true when make generate writes the file.
	Generated bool
}

// FixtureRelPath is the FND-001 generated fixture (not a public surface).
const FixtureRelPath = "testdata/generated/fixture.txt"

// PublicSurfaces is the closed first-GA release-diff set.
func PublicSurfaces() []Surface {
	return []Surface{
		{
			ID:          "openapi",
			Path:        "api/openapi/v1.json",
			Title:       "OpenAPI",
			ReviewToken: "OpenAPI",
			Generated:   true,
		},
		{
			ID:          "mcp",
			Path:        "api/mcp/v1.json",
			Title:       "MCP capability manifest",
			ReviewToken: "MCP capability manifest",
			Generated:   true,
		},
		{
			ID:          "config-schema",
			Path:        "api/jsonschema/labdns.dev.v1alpha1.json",
			Title:       "Configuration schema",
			ReviewToken: "Configuration schema",
			Generated:   false,
		},
		{
			ID:          "capabilities",
			Path:        "api/capabilities/v1.json",
			Title:       "Capability table",
			ReviewToken: "MCP capability manifest",
			Generated:   true,
		},
		{
			ID:          "metrics",
			Path:        "api/metrics/v1alpha1.json",
			Title:       "Metrics catalog",
			ReviewToken: "Metrics and labels",
			Generated:   true,
		},
		{
			ID:          "cli-help",
			Path:        "api/cli/help.txt",
			Title:       "CLI help",
			ReviewToken: "CLI flags and environment variables",
			Generated:   true,
		},
		{
			ID:          "error-catalog",
			Path:        "api/errors/v1.json",
			Title:       "Error code catalog",
			ReviewToken: "Error code catalog",
			Generated:   true,
		},
		{
			ID:          "chaos-effects",
			Path:        "api/chaos/effects.json",
			Title:       "Chaos action catalog",
			ReviewToken: "DNS record and chaos action support",
			Generated:   false,
		},
	}
}

// GeneratedRels returns slash-separated paths that make generate must write.
func GeneratedRels() []string {
	out := []string{FixtureRelPath}
	for _, s := range PublicSurfaces() {
		if s.Generated {
			out = append(out, s.Path)
		}
	}
	return out
}

// RequiredCIJobs are the GitHub Actions job IDs that must stay required.
// There is no optional or bypassable job in this list.
func RequiredCIJobs() []string {
	return []string{
		"format",
		"lint",
		"unit",
		"race",
		"fuzz-smoke",
		"generated-file",
		"documentation",
		"security-scan",
		"container-test",
		"changelog",
		"parity",
		"config-compat",
	}
}

// RequiredNoteHeadings are the ## sections every tag notes file must contain.
// Keep in lockstep with RELEASE-NOTES-TEMPLATE.md.
func RequiredNoteHeadings() []string {
	return []string{
		"Highlights",
		"Added",
		"Changed",
		"Fixed",
		"Removed or deprecated",
		"Security",
		"DNS behavior",
		"Chaos behavior",
		"REST API",
		"MCP API and protocol compatibility",
		"Configuration and schema",
		"Deployment and operations",
		"Observability",
		"Compatibility and migration",
		"Known limitations",
		"Complete functionality-difference review",
		"CI and release evidence",
	}
}

// TemplatePlaceholders are strings that mean the notes file is still a template.
func TemplatePlaceholders() []string {
	return []string{
		"YYYY-MM-DD",
		"PREVIOUS_VERSION",
		"LabDNS VERSION Release Notes",
		"Container digest: DIGEST",
		"Configuration versions: LIST",
		"REST versions: LIST",
		"MCP protocol versions: LIST",
		"Summarize the most important outcomes, not individual commits.",
		"List every new user-visible or operator-visible capability.",
	}
}
