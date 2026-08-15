package domainerr

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestNoWireMCPOrInternalImports(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
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
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
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
