// Command checkdocs verifies required root documents and internal markdown links.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RequiredRootDocs are the documents FND-001 must find at the repository root
// (plus the ingested architecture doc and implementation design).
var RequiredRootDocs = []string{
	"README.md",
	"AGENTS.md",
	"LICENSE",
	"SECURITY.md",
	"CHANGELOG.md",
	"CONTRIBUTING.md",
	"START-HERE.md",
	"RELEASE-NOTES-TEMPLATE.md",
	"CI-FAILURE-HARDENING-TEMPLATE.md",
	"MANIFEST.md",
	"Makefile",
	"go.mod",
	"docs/01-architecture.md",
	"docs/implementation-design.md",
	".github/CODEOWNERS",
	".github/pull_request_template.md",
	".github/workflows/ci.yml",
}

var mdLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkdocs: %v\n", err)
		os.Exit(1)
	}
	if err := Check(root); err != nil {
		fmt.Fprintf(os.Stderr, "checkdocs: %v\n", err)
		os.Exit(1)
	}
}

// Check verifies required documents exist and markdown internal links resolve.
func Check(root string) error {
	var missing []string
	for _, rel := range RequiredRootDocs {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required documents missing: %s", strings.Join(missing, ", "))
	}

	var broken []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "testdata" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		for _, m := range mdLink.FindAllSubmatch(body, -1) {
			target := strings.TrimSpace(string(m[1]))
			if i := strings.IndexAny(target, " \t"); i >= 0 {
				target = target[:i]
			}
			if skipLink(target) {
				continue
			}
			target = strings.SplitN(target, "#", 2)[0]
			if target == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(path), filepath.FromSlash(target))
			if _, err := os.Stat(resolved); err != nil {
				broken = append(broken, fmt.Sprintf("%s -> %s", rel, target))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(broken) > 0 {
		return fmt.Errorf("broken markdown links:\n  %s", strings.Join(broken, "\n  "))
	}
	return nil
}

func skipLink(target string) bool {
	switch {
	case target == "":
		return true
	case strings.HasPrefix(target, "#"):
		return true
	case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "mailto:"):
		return true
	default:
		return false
	}
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
