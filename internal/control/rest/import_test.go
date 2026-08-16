package rest

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestRESTImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"github.com/hilather/go-lab-dns/internal/app":           true,
		"github.com/hilather/go-lab-dns/internal/auth":          true,
		"github.com/hilather/go-lab-dns/internal/buildinfo":     true,
		"github.com/hilather/go-lab-dns/internal/capabilities":  true,
		"github.com/hilather/go-lab-dns/internal/config":        true,
		"github.com/hilather/go-lab-dns/internal/domainerr":     true,
		"github.com/hilather/go-lab-dns/internal/model":         true,
		"github.com/hilather/go-lab-dns/internal/observability": true,
		"github.com/hilather/go-lab-dns/internal/app":          true,
		"github.com/hilather/go-lab-dns/internal/audit":        true,
		"github.com/hilather/go-lab-dns/internal/auth":         true,
		"github.com/hilather/go-lab-dns/internal/buildinfo":    true,
		"github.com/hilather/go-lab-dns/internal/capabilities": true,
		"github.com/hilather/go-lab-dns/internal/config":       true,
		"github.com/hilather/go-lab-dns/internal/domainerr":    true,
		"github.com/hilather/go-lab-dns/internal/model":        true,
	}
	forbiddenPref := []string{
		"github.com/miekg/dns",
		"github.com/modelcontextprotocol",
		"github.com/coredns",
		"github.com/hilather/go-lab-dns/internal/compiler",
		"github.com/hilather/go-lab-dns/internal/snapshot",
		"github.com/hilather/go-lab-dns/internal/resolver",
		"github.com/hilather/go-lab-dns/internal/forwarder",
		"github.com/hilather/go-lab-dns/internal/chaos",
		"github.com/hilather/go-lab-dns/internal/dnsquery",
		"github.com/hilather/go-lab-dns/internal/dnsserver",
		"github.com/hilather/go-lab-dns/internal/dnswire",
		"github.com/hilather/go-lab-dns/internal/control/mcp",
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
