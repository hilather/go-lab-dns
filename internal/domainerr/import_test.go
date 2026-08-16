package domainerr

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestNoWireMCPOrInternalImports(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenPref := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/coredns",
		"github.com/hilather/go-lab-dns/internal/",
		"net/http",
		"net/http/pprof",
		"runtime/debug",
		"golang.org/x/net/dns",
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			isTest := strings.HasSuffix(filename, "_test.go")
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if isTest && path == "github.com/hilather/go-lab-dns/internal/testutil/goparse" {
					// Shared test-only helpers do not break production leaf purity.
					continue
				}
				for _, p := range forbiddenPref {
					root := strings.TrimSuffix(p, "/")
					if path == root || strings.HasPrefix(path, root+"/") {
						t.Errorf("%s imports forbidden %q", filename, path)
					}
				}
			}
		}
	}
}
