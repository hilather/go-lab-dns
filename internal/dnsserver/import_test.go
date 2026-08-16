package dnsserver

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestNoMiekgOrPolicyImports(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"github.com/miekg/dns",
		"github.com/hilather/go-lab-dns/internal/snapshot",
		"github.com/hilather/go-lab-dns/internal/resolver",
		"github.com/hilather/go-lab-dns/internal/forwarder",
		"github.com/hilather/go-lab-dns/internal/chaos",
		"github.com/hilather/go-lab-dns/internal/dnsquery",
		"github.com/hilather/go-lab-dns/internal/cache",
		"github.com/hilather/go-lab-dns/internal/compiler",
		"github.com/hilather/go-lab-dns/internal/config",
		"github.com/hilather/go-lab-dns/internal/app",
		"github.com/hilather/go-lab-dns/internal/control",
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, p := range forbidden {
					if path == p || strings.HasPrefix(path, p+"/") {
						t.Errorf("%s imports forbidden %q", filename, path)
					}
				}
			}
		}
	}
}

func TestRepoOnlyDnswireImportsMiekg(t *testing.T) {
	root := moduleRoot(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.Contains(filepath.ToSlash(rel), "internal/dnswire/") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == "github.com/miekg/dns" || strings.HasPrefix(p, "github.com/miekg/dns/") {
				t.Errorf("%s imports %q", rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
