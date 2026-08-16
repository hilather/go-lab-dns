package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
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

func TestGeneratedRelsCoverPublicSurfaces(t *testing.T) {
	root := mustRoot(t)
	files, err := plannedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range files {
		got[filepath.ToSlash(f.rel)] = true
	}
	for _, rel := range releasecontract.GeneratedRels() {
		if !got[rel] {
			t.Errorf("plannedFiles missing generated rel %s", rel)
		}
	}
	for _, s := range releasecontract.PublicSurfaces() {
		if s.Generated && !got[s.Path] {
			t.Errorf("plannedFiles missing public surface %s", s.Path)
		}
	}
}

func TestCheckFilesReportsEachStaleSurface(t *testing.T) {
	dir := t.TempDir()
	files := []artifact{
		{rel: "testdata/generated/fixture.txt", body: []byte("fixture-want\n")},
		{rel: "api/cli/help.txt", body: []byte("help-want\n")},
		{rel: "api/errors/v1.json", body: []byte("{}\n")},
	}
	for _, f := range files {
		path := filepath.Join(dir, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, f.body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkFiles(dir, files); err != nil {
		t.Fatalf("clean tree: %v", err)
	}
	for _, f := range files {
		t.Run(f.rel, func(t *testing.T) {
			path := filepath.Join(dir, filepath.FromSlash(f.rel))
			if err := os.WriteFile(path, append(append([]byte{}, f.body...), []byte("dirty\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
			err := checkFiles(dir, files)
			if err == nil {
				t.Fatal("expected stale")
			}
			if !strings.Contains(err.Error(), f.rel) {
				t.Fatalf("error %q does not name %s", err, f.rel)
			}
			if err := os.WriteFile(path, f.body, 0o644); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifyGeneratedClean(t *testing.T) {
	root := mustRoot(t)
	cmd := exec.Command("go", "run", "./scripts/generate", "-check")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected clean generated files; %v\n%s", err, out)
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
