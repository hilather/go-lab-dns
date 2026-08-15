package checktargets

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
}

var unimplementedTargets = []string{
	"test-integration",
	"test-parity",
	"test-config-compat",
	"test-container",
}

var requiredCIJobs = []string{
	"format",
	"lint",
	"unit",
	"race",
	"fuzz-smoke",
	"generated-file",
	"documentation",
	"security-scan",
	"container-test",
}

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

func isNoOpSuccess(recipe string) bool {
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
