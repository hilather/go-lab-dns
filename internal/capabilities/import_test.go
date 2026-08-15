package capabilities

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"github.com/hilather/go-lab-dns/internal/domainerr": true,
	}
	forbiddenPref := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/coredns",
		"github.com/hilather/go-lab-dns/internal/app",
		"github.com/hilather/go-lab-dns/internal/control",
		"github.com/hilather/go-lab-dns/internal/dnsserver",
		"github.com/hilather/go-lab-dns/internal/dnswire",
		"github.com/hilather/go-lab-dns/internal/dnsquery",
		"net/http",
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, p := range forbiddenPref {
					if path == p || strings.HasPrefix(path, p+"/") {
						t.Errorf("%s imports forbidden %q", filename, path)
					}
				}
				if !strings.HasPrefix(path, "github.com/hilather/go-lab-dns/internal/") {
					continue
				}
				if !allowed[path] {
					t.Errorf("%s production import %q", filename, path)
				}
			}
		}
	}
}
