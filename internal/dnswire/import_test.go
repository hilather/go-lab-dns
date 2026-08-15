package dnswire

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiekgDNSOnlyImportedHere(t *testing.T) {
	root := moduleRoot(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.Contains(filepath.ToSlash(rel), "internal/dnswire/") {
			return nil
		}
		imports := fileImports(t, path)
		for _, imp := range imports {
			if imp == "github.com/miekg/dns" || strings.HasPrefix(imp, "github.com/miekg/dns/") {
				t.Errorf("%s imports %q; miekg/dns must stay in internal/dnswire", rel, imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDnswireImportsOnlyModelAndMiekg(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.Name, "_test") {
			continue
		}
		for filename, f := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if !strings.HasPrefix(path, "github.com/") && !strings.HasPrefix(path, "golang.org/") {
					continue
				}
				switch {
				case path == "github.com/miekg/dns":
				case path == "github.com/hilather/go-lab-dns/internal/model":
				default:
					t.Errorf("%s imports %q; dnswire production code may import only model and miekg/dns", filename, path)
				}
			}
		}
	}
}

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
		return nil
	}
	var out []string
	for _, imp := range f.Imports {
		out = append(out, strings.Trim(imp.Path.Value, `"`))
	}
	return out
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
