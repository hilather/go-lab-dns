package auth

import (
	"net/http"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestCheckOrigin(t *testing.T) {
	if err := CheckOrigin("", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("http://127.0.0.1:8080", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("http://localhost", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("https://evil.example", nil); err == nil {
		t.Fatal("evil allowed")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeForbidden {
		t.Fatalf("err=%v", err)
	}
	if err := CheckOrigin("https://ok.example", []string{"https://ok.example"}); err != nil {
		t.Fatal(err)
	}
	if OriginAllowed("file://localhost", nil) {
		t.Fatal("file")
	}
}

func TestApplyCORSWritesNothing(t *testing.T) {
	h := make(http.Header)
	ApplyCORS(h)
	if h.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal(h)
	}
}
