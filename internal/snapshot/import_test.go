package snapshot

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestImportsOnlyStdlibAndModel(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowedInternal := "github.com/hilather/go-lab-dns/internal/model"
	allowedTestInternal := "github.com/hilather/go-lab-dns/internal/testutil"
	forbiddenPref := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/coredns",
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			testFile := strings.HasSuffix(filename, "_test.go")
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, p := range forbiddenPref {
					if path == p || strings.HasPrefix(path, p+"/") {
						t.Errorf("%s imports forbidden %q", filename, path)
					}
				}
				if !strings.HasPrefix(path, "github.com/hilather/go-lab-dns/") {
					continue
				}
				if path == allowedInternal {
					continue
				}
				if testFile && path == allowedTestInternal {
					continue
				}
				t.Errorf("%s imports %q; snapshot production code may import only model", filename, path)
			}
		}
	}
}
