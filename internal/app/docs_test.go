package app

import (
	"bytes"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestDocsOutsideSourceTree(t *testing.T) {
	svc := New(Options{})
	t.Chdir(t.TempDir())
	for _, id := range []string{"dns-semantics", "chaos-safety"} {
		body, err := svc.Docs(t.Context(), actor(), id)
		if err != nil {
			t.Fatalf("%s without source checkout: %v", id, err)
		}
		if !bytes.HasPrefix(body, []byte("# ")) {
			t.Fatalf("%s is not Markdown", id)
		}
		body[0] = '!'
		again, err := svc.Docs(t.Context(), actor(), id)
		if err != nil || !bytes.HasPrefix(again, []byte("# ")) {
			t.Fatalf("caller altered embedded %s: %s, %v", id, again, err)
		}
	}
	_, err := svc.Docs(t.Context(), actor(), "../../go.mod")
	requireCode(t, err, domainerr.CodeNotFound)
}
