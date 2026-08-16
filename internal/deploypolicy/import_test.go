package deploypolicy

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestDeploypolicyImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/hilather/go-lab-dns/internal/app",
		"github.com/hilather/go-lab-dns/internal/control",
		"github.com/hilather/go-lab-dns/internal/dnsquery",
		"github.com/hilather/go-lab-dns/internal/dnsserver",
		"github.com/hilather/go-lab-dns/internal/snapshot",
		"net/http",
	}
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, p := range forbidden {
					if path == p || strings.HasPrefix(path, p+"/") {
						t.Errorf("%s imports forbidden %q", filename, path)
					}
				}
			}
		}
	}
}
