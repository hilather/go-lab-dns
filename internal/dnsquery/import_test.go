package dnsquery

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestNoMiekgAndImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowedProd := map[string]bool{
		"github.com/hilather/go-lab-dns/internal/chaos":         true,
		"github.com/hilather/go-lab-dns/internal/chaos/effects": true,
		"github.com/hilather/go-lab-dns/internal/model":         true,
		"github.com/hilather/go-lab-dns/internal/snapshot":      true,
		"github.com/hilather/go-lab-dns/internal/resolver":      true,
		"github.com/hilather/go-lab-dns/internal/forwarder":     true,
		"github.com/hilather/go-lab-dns/internal/cache":         true,
		"github.com/hilather/go-lab-dns/internal/dnsserver":     true,
		"github.com/hilather/go-lab-dns/internal/observability": true,
		"github.com/hilather/go-lab-dns/internal/testutil":      true,
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
					t.Errorf("%s production import %q", filename, path)
				}
			}
		}
	}
}
