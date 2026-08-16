package config

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestConfigImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/coredns",
		"github.com/hilather/go-lab-dns/internal/snapshot",
		"github.com/hilather/go-lab-dns/internal/resolver",
		"github.com/hilather/go-lab-dns/internal/forwarder",
		"github.com/hilather/go-lab-dns/internal/compiler",
		"github.com/hilather/go-lab-dns/internal/app",
		"github.com/hilather/go-lab-dns/internal/control",
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
