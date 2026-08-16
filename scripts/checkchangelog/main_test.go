package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckFailsWithoutChangelog(t *testing.T) {
	root := initRepo(t)
	base := revParse(t, root, "HEAD")
	write(t, root, "api/openapi/v1.json", "{}\n")
	commit(t, root, "openapi without changelog")
	err := Check(root, base)
	if err == nil || !strings.Contains(err.Error(), "CHANGELOG.md") {
		t.Fatalf("want changelog failure, got %v", err)
	}
}

func TestCheckPassesWithChangelog(t *testing.T) {
	root := initRepo(t)
	base := revParse(t, root, "HEAD")
	write(t, root, "api/openapi/v1.json", "{}\n")
	write(t, root, "CHANGELOG.md", "## Unreleased\n- OpenAPI change\n")
	commit(t, root, "openapi with changelog")
	if err := Check(root, base); err != nil {
		t.Fatal(err)
	}
}

func TestCheckIgnoresDocsOnlyAndTests(t *testing.T) {
	root := initRepo(t)
	base := revParse(t, root, "HEAD")
	write(t, root, "docs/01-architecture.md", "# x\n")
	write(t, root, "internal/app/foo_test.go", "package app\n")
	write(t, root, "tasks/15-ci-docs-release.md", "# t\n")
	commit(t, root, "docs and tests")
	if err := Check(root, base); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSeesUncommittedChangelog(t *testing.T) {
	root := initRepo(t)
	base := revParse(t, root, "HEAD")
	write(t, root, "cmd/labdns/main.go", "package main\n")
	write(t, root, "CHANGELOG.md", "## Unreleased\n- CLI\n")
	// Do not commit: local `make test-changelog` must still pass.
	if err := Check(root, base); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRepoAgainstMainIfPresent(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	base := "origin/main"
	if _, err := gitLines(root, "rev-parse", "--verify", base); err != nil {
		base = "main"
		if _, err := gitLines(root, "rev-parse", "--verify", base); err != nil {
			t.Skip("no main ref")
		}
	}
	if err := Check(root, base); err != nil {
		t.Fatal(err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, c := range [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "ci@labdns.dev"},
		{"git", "config", "user.name", "labdns-ci"},
		{"git", "config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	write(t, dir, "README", "t\n")
	commit(t, dir, "init")
	return dir
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, root, msg string) {
	t.Helper()
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func revParse(t *testing.T, root, rev string) string {
	t.Helper()
	lines, err := gitLines(root, "rev-parse", rev)
	if err != nil || len(lines) == 0 {
		t.Fatalf("rev-parse: %v %v", err, lines)
	}
	return lines[0]
}
