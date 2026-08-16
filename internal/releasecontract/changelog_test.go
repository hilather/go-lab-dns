package releasecontract

import "testing"

func TestCheckChangelogRequiresEntryForObservablePaths(t *testing.T) {
	err := CheckChangelog([]string{"api/openapi/v1.json", "cmd/labdns/main.go"})
	if err == nil {
		t.Fatal("expected missing changelog")
	}
	if err := CheckChangelog([]string{"api/openapi/v1.json", ChangelogRel}); err != nil {
		t.Fatal(err)
	}
	if err := CheckChangelog([]string{"docs/01-architecture.md", "tasks/15-ci-docs-release.md"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckChangelog([]string{"internal/app/service_test.go"}); err != nil {
		t.Fatal(err)
	}
}

func TestObservableRel(t *testing.T) {
	cases := map[string]bool{
		"api/cli/help.txt":               true,
		"cmd/labdns/main.go":             true,
		"internal/clihelp/help.go":       true,
		"internal/clihelp/help_test.go":  false,
		"CHANGELOG.md":                   false,
		"docs/14-release-engineering.md": true,
		"docs/01-architecture.md":        false,
		"tasks/15-ci-docs-release.md":    false,
		"Dockerfile":                     true,
		".github/workflows/ci.yml":       true,
	}
	for rel, want := range cases {
		if got := ObservableRel(rel); got != want {
			t.Errorf("ObservableRel(%q)=%v want %v", rel, got, want)
		}
	}
}
