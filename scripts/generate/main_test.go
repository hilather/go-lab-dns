package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderContainsModuleAndHash(t *testing.T) {
	root := mustRoot(t)
	got, err := Render(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "module:github.com/hilather/go-lab-dns") {
		t.Fatalf("missing module line:\n%s", got)
	}
	if !strings.Contains(got, "sha256:") {
		t.Fatalf("missing sha256:\n%s", got)
	}
	if !strings.HasPrefix(got, header) {
		t.Fatalf("missing header:\n%s", got)
	}
}

func TestVerifyGeneratedFailsWhenDirty(t *testing.T) {
	root := mustRoot(t)
	fixture := filepath.Join(root, "testdata", "generated", "fixture.txt")
	orig, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(fixture, orig, 0o644); err != nil {
			t.Errorf("restore fixture: %v", err)
		}
	})

	dirty := append([]byte(nil), orig...)
	dirty = append(dirty, []byte("intentionally-dirty\n")...)
	if err := os.WriteFile(fixture, dirty, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./scripts/generate", "-check")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected dirty fixture to fail -check; output=%s", out)
	}

	cmd = exec.Command("go", "run", "./scripts/generate")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}

	cmd = exec.Command("go", "run", "./scripts/generate", "-check")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected clean fixture after generate; %v\n%s", err, out)
	}
}

func mustRoot(t *testing.T) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
