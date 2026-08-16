package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

func TestCompareDetectsAddedChangedRemoved(t *testing.T) {
	root := initRepo(t)
	writeSurface(t, root, "openapi", `{"openapi":"3.1.0","info":{"title":"old"}}`+"\n")
	writeSurface(t, root, "mcp", `{"protocol":"2026-07-28"}`+"\n")
	commit(t, root, "base surfaces")
	base := revParse(t, root, "HEAD")

	writeSurface(t, root, "openapi", `{"openapi":"3.1.0","info":{"title":"new"}}`+"\n")
	writeSurface(t, root, "cli-help", "usage: labdns serve --new-flag\n")
	// drop mcp
	if err := os.Remove(surfacePath(root, "mcp")); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "change surfaces")
	head := revParse(t, root, "HEAD")

	rep, err := Compare(root, base, head)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Status{}
	for _, s := range rep.Surfaces {
		got[s.ID] = s.Status
	}
	if got["openapi"] != StatusChanged {
		t.Errorf("openapi=%s want changed", got["openapi"])
	}
	if got["cli-help"] != StatusAdded {
		t.Errorf("cli-help=%s want added", got["cli-help"])
	}
	if got["mcp"] != StatusRemoved {
		t.Errorf("mcp=%s want removed", got["mcp"])
	}
	if got["metrics"] != StatusMissing {
		t.Errorf("metrics=%s want missing-both", got["metrics"])
	}
	changed := rep.ChangedSurfaces()
	ids := map[string]bool{}
	for _, s := range changed {
		ids[s.ID] = true
	}
	if !ids["openapi"] || !ids["cli-help"] || !ids["mcp"] {
		t.Fatalf("changed=%v", ids)
	}
	if ids["metrics"] {
		t.Fatal("missing-both should not count as a curated change")
	}
}

func TestRunDiffFailsOnDirtyWorktree(t *testing.T) {
	root := initRepo(t)
	writeSurface(t, root, "openapi", "{}\n")
	commit(t, root, "one")
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runDiff(root, "HEAD", "HEAD", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "dirty worktree") {
		t.Fatalf("want dirty worktree, got %v", err)
	}
	if err := runDiff(root, "HEAD", "HEAD", "", false, true); err != nil {
		t.Fatal(err)
	}
}

func TestRunDiffFailsWhenCLIHelpUndocumented(t *testing.T) {
	root := initRepo(t)
	writeSurface(t, root, "cli-help", "usage: labdns old\n")
	commit(t, root, "old help")
	base := revParse(t, root, "HEAD")
	writeSurface(t, root, "cli-help", "usage: labdns --new-flag\n")
	commit(t, root, "new help")

	notes := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(notes, []byte(unaccountedNotes()), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runDiff(root, base, "HEAD", notes, false, false)
	if err == nil || !strings.Contains(err.Error(), "cli-help") {
		t.Fatalf("want undocumented cli-help, got %v", err)
	}

	if err := os.WriteFile(notes, []byte(accountedNotes()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runDiff(root, base, "HEAD", notes, false, false); err != nil {
		t.Fatal(err)
	}
}

func TestNotesOnlyRejectsIncomplete(t *testing.T) {
	root := initRepo(t)
	notes := filepath.Join(root, "RELEASE-NOTES-TEMPLATE.md")
	// Copy the real template so headings exist but placeholders remain.
	real, err := os.ReadFile(filepath.Join(mustRoot(t), "RELEASE-NOTES-TEMPLATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notes, real, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "-notes-only", "-notes", notes)
	cmd.Dir = filepath.Join(mustRoot(t), "scripts", "release-diff")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("template notes-only must fail; out=%s", out)
	}
	if !strings.Contains(string(out), "incomplete release notes") && !strings.Contains(string(out), "placeholder") {
		t.Fatalf("output=%s", out)
	}
}

func TestRequireCIFixture(t *testing.T) {
	root := mustRoot(t)
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.json")
	fail := filepath.Join(dir, "fail.json")
	jobs := releasecontract.RequiredCIJobs()
	writeFixture(t, ok, jobs, "success")
	writeFixture(t, fail, jobs, "failure")

	sha, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", ".", "-require-ci", "-ci-fixture", ok)
	cmd.Dir = filepath.Join(root, "scripts", "release-diff")
	cmd.Env = withEnv(os.Environ(), "GITHUB_SHA="+sha)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ok fixture: %v\n%s", err, out)
	}

	cmd = exec.Command("go", "run", ".", "-require-ci", "-ci-fixture", fail)
	cmd.Dir = filepath.Join(root, "scripts", "release-diff")
	cmd.Env = withEnv(os.Environ(), "GITHUB_SHA="+sha)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("failed fixture must block the tag")
	}
	if !strings.Contains(string(out), "cannot proceed") {
		t.Fatalf("output=%s", out)
	}
}

func TestPreviousTagFallsBackToEmptyTree(t *testing.T) {
	root := initRepo(t)
	tag, err := previousTagName(root)
	if err != nil {
		t.Fatal(err)
	}
	if tag != emptyTreeSHA {
		t.Fatalf("previous tag=%s want empty tree", tag)
	}
	if err := exec.Command("git", "-C", root, "tag", "v0.0.1").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "next.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "after tag")
	tag, err = previousTagName(root)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.0.1" {
		t.Fatalf("previous tag=%s", tag)
	}
}

func TestCompareRejectsUnknownRef(t *testing.T) {
	root := initRepo(t)
	_, err := Compare(root, "definitely-not-a-ref", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "unknown git ref") {
		t.Fatalf("want unknown git ref, got %v", err)
	}
}

func TestCompareAgainstEmptyTreeReportsAdds(t *testing.T) {
	root := initRepo(t)
	writeSurface(t, root, "openapi", "{}\n")
	commit(t, root, "add openapi")
	rep, err := Compare(root, emptyTreeSHA, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var openapi SurfaceDelta
	for _, s := range rep.Surfaces {
		if s.ID == "openapi" {
			openapi = s
		}
	}
	if openapi.Status != StatusAdded {
		t.Fatalf("openapi=%s want added", openapi.Status)
	}
}

func writeFixture(t *testing.T, path string, jobs []string, conclusion string) {
	t.Helper()
	sha, err := gitOutput(mustRoot(t), "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var runs []releasecontract.CheckRun
	for _, j := range jobs {
		runs = append(runs, releasecontract.CheckRun{
			Name: j, Status: "completed", Conclusion: conclusion, HeadSHA: sha,
		})
	}
	raw, err := json.Marshal(fixtureFile{CheckRuns: runs})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func unaccountedNotes() string {
	var b strings.Builder
	b.WriteString("# LabDNS 0.0.0-test Release Notes\n\n")
	b.WriteString("Release date: 2026-08-15\nPrevious release: none\nApplication version: 0.0.0-test\n")
	b.WriteString("Configuration versions: labdns.dev/v1alpha1\nREST versions: /v1\n")
	b.WriteString("MCP protocol versions: 2026-07-28\nContainer digest: sha256:test\n\n")
	for _, h := range releasecontract.RequiredNoteHeadings() {
		b.WriteString("## ")
		b.WriteString(h)
		b.WriteByte('\n')
		if h == "Complete functionality-difference review" {
			for _, s := range releasecontract.PublicSurfaces() {
				mark := "x"
				if s.ID == "cli-help" {
					mark = " "
				}
				b.WriteString("- [")
				b.WriteString(mark)
				b.WriteString("] ")
				b.WriteString(s.ReviewToken)
				b.WriteByte('\n')
			}
		} else {
			b.WriteString("Curated notes without mentioning the CLI flag.\n")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func accountedNotes() string {
	var b strings.Builder
	b.WriteString("# LabDNS 0.0.0-test Release Notes\n\n")
	b.WriteString("Release date: 2026-08-15\nPrevious release: none\nApplication version: 0.0.0-test\n")
	b.WriteString("Configuration versions: labdns.dev/v1alpha1\nREST versions: /v1\n")
	b.WriteString("MCP protocol versions: 2026-07-28\nContainer digest: sha256:test\n\n")
	for _, h := range releasecontract.RequiredNoteHeadings() {
		b.WriteString("## ")
		b.WriteString(h)
		b.WriteByte('\n')
		if h == "Deployment and operations" {
			b.WriteString("CLI help gained --new-flag.\n")
		}
		if h == "Complete functionality-difference review" {
			b.WriteString("Reviewed CLI help (cli-help) against api/cli/help.txt.\n")
			for _, s := range releasecontract.PublicSurfaces() {
				b.WriteString("- [x] ")
				b.WriteString(s.ReviewToken)
				b.WriteByte('\n')
			}
		} else if h != "Deployment and operations" {
			b.WriteString("Curated notes.\n")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "ci@labdns.dev"},
		{"git", "config", "user.name", "labdns-ci"},
		{"git", "config", "commit.gpgsign", "false"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "init")
	return dir
}

func writeSurface(t *testing.T, root, id, body string) {
	t.Helper()
	var path string
	for _, s := range releasecontract.PublicSurfaces() {
		if s.ID == id {
			path = s.Path
			break
		}
	}
	if path == "" {
		t.Fatalf("unknown surface %s", id)
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func surfacePath(root, id string) string {
	for _, s := range releasecontract.PublicSurfaces() {
		if s.ID == id {
			return filepath.Join(root, filepath.FromSlash(s.Path))
		}
	}
	return ""
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
	out, err := gitOutput(root, "rev-parse", rev)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func withEnv(env []string, kv string) []string {
	key, _, _ := strings.Cut(kv, "=")
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, kv)
}

func mustRoot(t *testing.T) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
