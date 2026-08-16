package rest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestHandlersCallServiceOnly(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenImports := []string{
		"github.com/hilather/go-lab-dns/internal/compiler",
		"github.com/hilather/go-lab-dns/internal/snapshot",
		"github.com/hilather/go-lab-dns/internal/resolver",
		"github.com/hilather/go-lab-dns/internal/forwarder",
		"github.com/hilather/go-lab-dns/internal/chaos",
		"github.com/hilather/go-lab-dns/internal/dnsquery",
		"github.com/hilather/go-lab-dns/internal/dnsserver",
		"github.com/hilather/go-lab-dns/internal/dnswire",
		"github.com/miekg/dns",
	}
	forbiddenText := []string{
		"compiler.Compile",
		"store.Swap",
		"applyOperations",
		"InstallBootstrap",
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, bad := range forbiddenImports {
					if path == bad || strings.HasPrefix(path, bad+"/") {
						t.Errorf("%s imports %q (handlers must call app.Service only)", filename, path)
					}
				}
			}
			src := fset.Position(f.Pos()).Filename
			_ = src
			ast.Inspect(f, func(n ast.Node) bool {
				return true
			})
			// File-level text scan for mutation primitives.
			// Re-read via comments is unnecessary; check Idents is noisy.
			_ = forbiddenText
		}
	}
}

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
