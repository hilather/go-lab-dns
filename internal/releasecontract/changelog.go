package releasecontract

import (
	"fmt"
	"path"
	"strings"
)

// ChangelogRel is the curated changelog path.
const ChangelogRel = "CHANGELOG.md"

// ObservableRel reports whether a repository-relative path is externally
// observable and therefore requires a changelog entry.
func ObservableRel(rel string) bool {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if rel == ChangelogRel {
		return false
	}
	if strings.HasSuffix(rel, "_test.go") {
		return false
	}
	prefixes := []string{
		"api/",
		"cmd/",
		"internal/",
		"examples/",
		"scripts/",
		".github/workflows/",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	switch rel {
	case "Dockerfile", "Makefile", "README.md", "SECURITY.md", "AGENTS.md":
		return true
	}
	switch {
	case strings.HasPrefix(rel, "docs/06-"),
		strings.HasPrefix(rel, "docs/07-"),
		strings.HasPrefix(rel, "docs/09-"),
		strings.HasPrefix(rel, "docs/11-"),
		strings.HasPrefix(rel, "docs/13-"),
		strings.HasPrefix(rel, "docs/14-"),
		strings.HasPrefix(rel, "docs/16-"),
		strings.HasPrefix(rel, "docs/17-"):
		return true
	}
	return false
}

// CheckChangelog fails when observable paths changed and CHANGELOG.md did not.
func CheckChangelog(changed []string) error {
	var observable []string
	changelog := false
	for _, rel := range changed {
		rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
		if rel == ChangelogRel {
			changelog = true
			continue
		}
		if ObservableRel(rel) {
			observable = append(observable, rel)
		}
	}
	if len(observable) == 0 {
		return nil
	}
	if changelog {
		return nil
	}
	return fmt.Errorf("externally observable paths changed without a %s entry:\n  %s", ChangelogRel, strings.Join(observable, "\n  "))
}
