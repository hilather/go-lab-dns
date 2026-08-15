package model

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestNoWireOrMCPOrInternalImports(t *testing.T) {
	assertNoForbiddenImports(t, ".")
}

func assertNoForbiddenImports(t *testing.T, dir string) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	forbidden := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/coredns",
		"net/http",
		"net/http/pprof",
		"runtime/debug",
		"golang.org/x/net/dns",
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
				if strings.HasPrefix(path, "github.com/hilather/go-lab-dns/internal/") {
					t.Errorf("%s imports internal package %q (model must stay leaf)", filename, path)
				}
			}
		}
	}
}
