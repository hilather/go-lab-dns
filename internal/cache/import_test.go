package cache

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestNoMiekgAndImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowedProd := map[string]bool{
		"github.com/hilather/go-lab-dns/internal/model":    true,
		"github.com/hilather/go-lab-dns/internal/testutil": true,
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			testFile := strings.HasSuffix(filename, "_test.go")
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == "github.com/miekg/dns" || strings.HasPrefix(path, "github.com/miekg/dns/") {
					t.Errorf("%s imports miekg", filename)
				}
				if !strings.HasPrefix(path, "github.com/hilather/go-lab-dns/internal/") {
					continue
				}
				if testFile {
					continue
				}
				if !allowedProd[path] {
					t.Errorf("%s production import %q; cache may import only model and testutil", filename, path)
				}
			}
		}
	}
}
