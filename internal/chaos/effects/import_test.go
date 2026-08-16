package effects

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestEffectsImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, ".", parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"github.com/hilather/go-lab-dns/internal/cache":     true,
		"github.com/hilather/go-lab-dns/internal/chaos":     true,
		"github.com/hilather/go-lab-dns/internal/dnsserver": true,
		"github.com/hilather/go-lab-dns/internal/forwarder": true,
		"github.com/hilather/go-lab-dns/internal/model":     true,
		"github.com/hilather/go-lab-dns/internal/snapshot":  true,
		"github.com/hilather/go-lab-dns/internal/testutil":  true,
	}
	forbidden := []string{
		"github.com/miekg/dns",
		"github.com/hilather/go-lab-dns/internal/app",
		"github.com/hilather/go-lab-dns/internal/dnsquery",
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
