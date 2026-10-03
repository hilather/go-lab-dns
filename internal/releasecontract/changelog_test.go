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

func TestChangelogIncludesApplicationAndDependencyChanges(t *testing.T) {
	for _, rel := range []string{"web/src/App.tsx", "web/src/style.css", "web/package.json", "web/package-lock.json", "web/vite.config.ts", "web/public/logo.svg", "go.mod", "go.sum"} {
		if err := CheckChangelog([]string{rel}); err == nil {
			t.Errorf("%s escaped changelog gate", rel)
		}
	}
	for _, rel := range []string{"web/src/App.test.tsx", "web/src/api.test.ts", "web/e2e/auth.spec.ts", "web/playwright.config.ts", "web/vitest.config.ts", "web/test/setup.ts", "web/dist/assets/index.js"} {
		if err := CheckChangelog([]string{rel}); err != nil {
			t.Errorf("test/build-only %s requires notes: %v", rel, err)
		}
	}
}
