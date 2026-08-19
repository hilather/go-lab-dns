package checktargets

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

// Required by AGENTS.md. Each must be a real check or an explicit unimplemented failure.
var requiredTargets = []string{
	"format",
	"lint",
	"generate",
	"verify-generated",
	"test",
	"test-race",
	"test-fuzz-smoke",
	"test-integration",
	"test-parity",
	"test-config-compat",
	"test-docs",
	"test-container",
	"security-scan",
	"test-changelog",
	"release-diff",
	"web-test",
	"web-build",
}

var unimplementedTargets = []string{}

// unimplementedCIJobs locks fail-closed CI jobs to an invert+phrase contract.
// Remove a job from this map in the same change that makes its Make target succeed.
var unimplementedCIJobs = map[string]unimplementedCIJob{}

type unimplementedCIJob struct {
	Target string
	Phrase string
}

var requiredCIJobs = releasecontract.RequiredCIJobs()

var targetHeader = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+):`)

func TestRequiredMakeTargetsAreNotNoOp(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	targets := parseTargets(string(body))
	for _, name := range requiredTargets {
		recipe, ok := targets[name]
		if !ok {
			t.Errorf("missing required target %s", name)
			continue
		}
		if isNoOpSuccess(recipe) {
			t.Errorf("target %s is a no-op success:\n%s", name, recipe)
		}
	}
}

func TestUnimplementedTargetsFailClosed(t *testing.T) {
	root := mustRoot(t)
	for _, name := range unimplementedTargets {
		cmd := exec.Command("make", name)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s succeeded; want fail-closed unimplemented\n%s", name, out)
			continue
		}
		if !bytes.Contains(bytes.ToLower(out), []byte("unimplemented")) {
			t.Errorf("%s output does not mention unimplemented:\n%s", name, out)
		}
	}
}

func TestCIJobsPresent(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, job := range requiredCIJobs {
		if !strings.Contains(text, "\n  "+job+":") {
			t.Errorf("CI workflow missing job %q", job)
		}
		jobBody, ok := yamlJobBody(text, job)
		if !ok {
			continue
		}
		if strings.Contains(jobBody, "continue-on-error:") {
			t.Errorf("required job %s must not set continue-on-error", job)
		}
		if strings.Contains(jobBody, "|| true") {
			t.Errorf("required job %s must not ignore failures with || true", job)
		}
		if jobLevelIf.MatchString(jobBody) {
			t.Errorf("required job %s must not have a job-level if: (skipped required jobs are a bypass)", job)
		}
	}
	gotJobs := yamlJobIDs(text)
	if len(gotJobs) != len(requiredCIJobs) {
		t.Errorf("ci.yml jobs=%v want exactly RequiredCIJobs=%v", gotJobs, requiredCIJobs)
	}
	want := map[string]bool{}
	for _, j := range requiredCIJobs {
		want[j] = true
	}
	for _, j := range gotJobs {
		if !want[j] {
			t.Errorf("ci.yml has extra job %q; required CI must not grow optional jobs", j)
		}
	}
}

var jobLevelIf = regexp.MustCompile(`(?m)^    if:`)

func yamlJobIDs(workflow string) []string {
	i := strings.Index(workflow, "\njobs:")
	if i < 0 {
		return nil
	}
	rest := workflow[i+len("\njobs:"):]
	re := regexp.MustCompile(`(?m)^  ([A-Za-z0-9_.-]+):`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(rest, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestReleaseWorkflowGatesTags(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"name: Release",
		"tag-gate:",
		"./scripts/release-diff",
		"-require-ci",
		"-notes",
		"fetch-depth: 0",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("release workflow missing %q", want)
		}
	}
	if strings.Contains(text, "continue-on-error:") {
		t.Error("release workflow must not set continue-on-error")
	}
	job, ok := yamlJobBody(text, "tag-gate")
	if !ok {
		t.Fatal("missing tag-gate job")
	}
	if strings.Contains(job, "|| true") {
		t.Error("tag-gate must not ignore failures")
	}
}

func TestWorkflowsForbidBroadRetries(t *testing.T) {
	root := mustRoot(t)
	dir := filepath.Join(root, ".github", "workflows")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	retry := regexp.MustCompile(`(?i)(max-attempts|retry-on|retry:)\s*:`)
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if retry.MatchString(text) {
			t.Errorf("%s defines a retry policy; only a documented bounded external transient is allowed", e.Name())
		}
		if strings.Contains(text, "continue-on-error: true") {
			t.Errorf("%s sets continue-on-error: true", e.Name())
		}
	}
}

func TestUnimplementedTargetRecipesAreOnlyFailClosed(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	targets := parseTargets(string(body))
	for _, name := range unimplementedTargets {
		recipe, ok := targets[name]
		if !ok {
			t.Errorf("missing unimplemented target %s", name)
			continue
		}
		cmds := recipeCommands(recipe)
		if len(cmds) != 2 {
			t.Errorf("%s recipe must be echo + exit 1 only (got %d commands):\n%s", name, len(cmds), recipe)
			continue
		}
		if !strings.HasPrefix(cmds[0], "echo ") || !strings.Contains(cmds[0], "unimplemented") {
			t.Errorf("%s first command must echo unimplemented, got %q", name, cmds[0])
		}
		if cmds[1] != "exit 1" {
			t.Errorf("%s second command must be exit 1, got %q", name, cmds[1])
		}
	}
}

func TestWebTargetsFailClosedAndStayVitestOnly(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	targets := parseTargets(string(body))
	for _, name := range []string{"web-test", "web-build", "web-generate"} {
		recipe, ok := targets[name]
		if !ok {
			t.Errorf("missing target %s", name)
			continue
		}
		if !strings.Contains(recipe, "command -v") {
			t.Errorf("%s must fail closed when node is missing", name)
		}
		if strings.Contains(recipe, "exit 0") {
			t.Errorf("%s must not exit 0 as a placeholder", name)
		}
		if strings.Contains(strings.ToLower(recipe), "playwright") {
			t.Errorf("%s must not invoke Playwright", name)
		}
	}
	build := targets["web-build"]
	if strings.Contains(build, "internal/web") {
		t.Error("web-build must not copy into or rm internal/web/dist")
	}
}

func TestWebNestedModuleFence(t *testing.T) {
	root := mustRoot(t)
	mod, err := os.ReadFile(filepath.Join(root, "web", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mod), "module github.com/hilather/go-lab-dns/web") {
		t.Fatalf("web/go.mod missing nested module fence:\n%s", mod)
	}
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list ./...: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "node_modules") {
			t.Errorf("parent go list walked node_modules: %s", line)
		}
		if strings.HasPrefix(line, "github.com/hilather/go-lab-dns/web") {
			t.Errorf("parent go list included nested web module: %s", line)
		}
	}
}

func TestWebCIJobHasNoPlaywright(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	job, ok := yamlJobBody(string(body), "web")
	if !ok {
		t.Fatal("CI workflow missing web job")
	}
	if strings.Contains(strings.ToLower(job), "playwright") {
		t.Error("web job must not install or run Playwright")
	}
	if !strings.Contains(job, "make web-test") {
		t.Error("web job must run make web-test")
	}
	if !strings.Contains(job, "make web-build") {
		t.Error("web job must run make web-build")
	}
	if !strings.Contains(job, "setup-node") {
		t.Error("web job must use setup-node")
	}
	if !strings.Contains(job, "22.14.0") {
		t.Error("web job must pin Node 22.14.0")
	}
}

func TestContainerTestJobInvertContract(t *testing.T) {
	root := mustRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	job, ok := yamlJobBody(text, "container-test")
	if !ok {
		t.Fatal("CI workflow missing container-test job body")
	}
	spec, inverted := unimplementedCIJobs["container-test"]
	if !inverted {
		if !strings.Contains(job, "make test-container") {
			t.Error("implemented container-test job must run make test-container")
		}
		if !strings.Contains(job, "setup-go") && !strings.Contains(strings.ToLower(job), "docker") {
			t.Error("implemented container-test job must use setup-go or Docker")
		}
		if strings.Contains(job, "|| true") {
			t.Error("implemented container-test job must not ignore Make failure with || true")
		}
		if hasStatusZeroInvert(job) {
			t.Error("implemented container-test job must not invert Make success into failure")
		}
		return
	}
	if !strings.Contains(job, spec.Phrase) {
		t.Errorf("container-test job must match exact phrase %q until %s is implemented", spec.Phrase, spec.Target)
	}
	if !hasStatusZeroInvert(job) {
		t.Error("container-test job must fail when make test-container exits 0 (invert contract)")
	}
	if !strings.Contains(job, "grep -q '"+spec.Phrase+"'") && !strings.Contains(job, `grep -q "`+spec.Phrase+`"`) {
		t.Errorf("container-test job must grep -q the exact phrase %q; do not treat a bare Make failure as success", spec.Phrase)
	}
	if strings.Contains(job, "|| true") {
		t.Error("container-test invert must not use || true")
	}
}

func parseTargets(makefile string) map[string]string {
	lines := strings.Split(makefile, "\n")
	out := make(map[string]string)
	var current string
	var buf []string
	flush := func() {
		if current != "" {
			out[current] = strings.Join(buf, "\n")
		}
		current = ""
		buf = nil
	}
	for _, line := range lines {
		if m := targetHeader.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line, "\t") && !strings.Contains(line, "=") {
			flush()
			current = m[1]
			continue
		}
		if current != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return out
}

func recipeCommands(recipe string) []string {
	var cmds []string
	for _, line := range strings.Split(recipe, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "@")
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmds = append(cmds, line)
	}
	return cmds
}

func yamlJobBody(workflow, job string) (string, bool) {
	header := "\n  " + job + ":"
	start := strings.Index(workflow, header)
	if start < 0 {
		if strings.HasPrefix(workflow, "  "+job+":") {
			start = 0
			header = "  " + job + ":"
		} else {
			return "", false
		}
	}
	rest := workflow[start+len(header):]
	next := regexp.MustCompile(`(?m)^  [A-Za-z0-9_.-]+:`)
	if loc := next.FindStringIndex(rest); loc != nil {
		rest = rest[:loc[0]]
	}
	return rest, true
}

func hasStatusZeroInvert(job string) bool {
	return strings.Contains(job, `"${status}" -eq 0`) ||
		strings.Contains(job, `${status} -eq 0`) ||
		strings.Contains(job, `"$status" -eq 0`)
}

func isNoOpSuccess(recipe string) bool {
	cmds := recipeCommands(recipe)
	if len(cmds) == 0 {
		return true
	}
	for _, c := range cmds {
		switch c {
		case "true", ":", "exit 0":
			continue
		default:
			if strings.HasPrefix(c, "echo ") && !strings.Contains(c, "exit") {
				continue
			}
			return false
		}
	}
	return true
}

func mustRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
