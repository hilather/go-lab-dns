package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestNoMutationPrimitivesInProduction(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	needles := []string{"compiler.Compile", ".Swap(", "applyOperations("}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			var b strings.Builder
			if err := ast.Fprint(&b, fset, f, ast.NotNilFilter); err != nil {
				t.Fatal(err)
			}
			txt := b.String()
			for _, n := range needles {
				if strings.Contains(txt, n) {
					t.Errorf("%s contains %q", filename, n)
				}
			}
		}
	}
}

func TestHealthNotRegisteredAsTools(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(tool.Name, "health") || tool.Name == "health.live" || tool.Name == "health.ready" {
			t.Fatalf("health probe leaked as tool %q", tool.Name)
		}
	}
}
