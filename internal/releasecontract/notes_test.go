package releasecontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateHeadingsMatchRequired(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "RELEASE-NOTES-TEMPLATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, h := range RequiredNoteHeadings() {
		if !hasHeading(text, h) {
			t.Errorf("RELEASE-NOTES-TEMPLATE.md missing ## %s", h)
		}
	}
}

func TestValidateNotesRejectsTemplateAndMissingSections(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "RELEASE-NOTES-TEMPLATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	r := ValidateNotes(string(raw), []Surface{{ID: "cli-help", Path: "api/cli/help.txt", Title: "CLI help", ReviewToken: "CLI flags and environment variables"}})
	if !r.Incomplete() {
		t.Fatal("template must be incomplete")
	}
	if len(r.Placeholders) == 0 {
		t.Fatal("expected leftover placeholders")
	}
	if len(r.Unaccounted) == 0 && len(r.UncheckedReviews) == 0 {
		t.Fatal("expected undocumented or unchecked CLI help")
	}

	r = ValidateNotes("# x\n", nil)
	if len(r.MissingHeadings) != len(RequiredNoteHeadings()) {
		t.Fatalf("missing headings=%d want %d", len(r.MissingHeadings), len(RequiredNoteHeadings()))
	}
}

func TestValidateNotesAcceptsFilledNotes(t *testing.T) {
	notes := filledNotes()
	r := ValidateNotes(notes, []Surface{{
		ID: "cli-help", Path: "api/cli/help.txt", Title: "CLI help",
		ReviewToken: "CLI flags and environment variables",
	}})
	if r.Incomplete() {
		t.Fatalf("filled notes rejected: %s", r.Error())
	}
}

func TestValidateNotesRequiresMentionOfChangedSurface(t *testing.T) {
	notes := filledNotes()
	// Drop the CLI mention and uncheck the box.
	notes = strings.ReplaceAll(notes, "CLI help now generated", "no cli mention here")
	notes = strings.ReplaceAll(notes, "- [x] CLI flags and environment variables", "- [ ] CLI flags and environment variables")
	r := ValidateNotes(notes, []Surface{{
		ID: "cli-help", Path: "api/cli/help.txt", Title: "CLI help",
		ReviewToken: "CLI flags and environment variables",
	}})
	if !r.Incomplete() {
		t.Fatal("expected incomplete notes after dropping CLI mention")
	}
	if len(r.Unaccounted) == 0 && len(r.UncheckedReviews) == 0 {
		t.Fatalf("expected unaccounted or unchecked cli-help, report=%s", r.Error())
	}
}

func filledNotes() string {
	var b strings.Builder
	b.WriteString("# LabDNS 0.0.0-test Release Notes\n\n")
	b.WriteString("Release date: 2026-08-15\n")
	b.WriteString("Previous release: none\n")
	b.WriteString("Application version: 0.0.0-test\n")
	b.WriteString("Configuration versions: labdns.dev/v1alpha1\n")
	b.WriteString("REST versions: /v1\n")
	b.WriteString("MCP protocol versions: 2026-07-28\n")
	b.WriteString("Container digest: sha256:test\n\n")
	for _, h := range RequiredNoteHeadings() {
		b.WriteString("## ")
		b.WriteString(h)
		b.WriteByte('\n')
		switch h {
		case "Complete functionality-difference review":
			b.WriteString("CLI help now generated as api/cli/help.txt.\n")
			for _, s := range PublicSurfaces() {
				b.WriteString("- [x] ")
				b.WriteString(s.ReviewToken)
				b.WriteByte('\n')
			}
		case "CI and release evidence":
			b.WriteString("- [x] All required CI passed on the exact tag commit.\n")
		default:
			b.WriteString("Curated notes for tests.\n")
		}
		b.WriteByte('\n')
	}
	return b.String()
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
