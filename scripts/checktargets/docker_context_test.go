package checktargets

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The documentation directory and all Markdown are otherwise excluded. Keep an
// exact final allowlist so the Docker COPY includes the package and every embed
// input without shipping unrelated documentation in the build context.
func TestDockerContextIncludesEmbeddedDocumentation(t *testing.T) {
	root := mustRoot(t)
	cmd := exec.Command("go", "list", "-json", "./docs")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect documentation package: %v\n%s", err, out)
	}
	var pkg struct {
		GoFiles    []string
		EmbedFiles []string
	}
	if err := json.Unmarshal(out, &pkg); err != nil {
		t.Fatal(err)
	}
	if len(pkg.GoFiles) == 0 || len(pkg.EmbedFiles) == 0 {
		t.Fatal("documentation package must contain Go sources and embedded documents")
	}
	paths := append(slices.Clone(pkg.GoFiles), pkg.EmbedFiles...)
	slices.Sort(paths)
	want := []string{"!docs/", "docs/*"}
	for _, name := range paths {
		want = append(want, "!docs/"+name)
	}
	body, err := os.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil {
		t.Fatal(err)
	}
	var rules []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			rules = append(rules, line)
		}
	}
	if len(rules) < len(want) || !slices.Equal(rules[len(rules)-len(want):], want) {
		t.Fatalf("Docker context must end with exact documentation package/embed allowlist:\n%s\nactual rules:\n%s", strings.Join(want, "\n"), strings.Join(rules, "\n"))
	}
}
