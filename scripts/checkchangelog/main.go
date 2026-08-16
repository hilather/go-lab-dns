// Command checkchangelog fails when observable paths change without CHANGELOG.md.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

func main() {
	base := flag.String("base", "", "git ref to diff against (default: CI base or origin/main)")
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkchangelog: %v\n", err)
		os.Exit(1)
	}
	ref := *base
	if ref == "" {
		ref = defaultBase()
	}
	if err := Check(root, ref); err != nil {
		fmt.Fprintf(os.Stderr, "checkchangelog: %v\n", err)
		os.Exit(1)
	}
}

// Check diffs root against base (committed + index + worktree) and requires
// a changelog entry when observable paths changed.
func Check(root, base string) error {
	if base == "" {
		return fmt.Errorf("base ref is empty")
	}
	changed, err := changedFiles(root, base)
	if err != nil {
		return err
	}
	return releasecontract.CheckChangelog(changed)
}

func changedFiles(root, base string) ([]string, error) {
	// Include committed, staged, and unstaged paths so a local run before
	// commit still sees the working changelog entry.
	queries := [][]string{
		{"diff", "--name-only", "--diff-filter=ACDMR", base},
		{"diff", "--name-only", "--cached", "--diff-filter=ACDMR", base},
		{"diff", "--name-only", "--diff-filter=ACDMR", base, "HEAD"},
	}
	seen := map[string]bool{}
	var out []string
	for _, q := range queries {
		names, err := gitLines(root, q...)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			n = filepath.ToSlash(n)
			if n == "" || seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

func defaultBase() string {
	if v := os.Getenv("GITHUB_BASE_REF"); v != "" {
		if !strings.HasPrefix(v, "origin/") {
			return "origin/" + v
		}
		return v
	}
	if v := os.Getenv("GITHUB_EVENT_BEFORE"); v != "" && v != strings.Repeat("0", 40) {
		return v
	}
	return "origin/main"
}

func gitLines(root string, args ...string) ([]string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	var lines []string
	for _, l := range strings.Split(stdout.String(), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
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
