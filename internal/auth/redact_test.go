package auth

import (
	"bytes"
	"testing"
)

func TestRedactJSON(t *testing.T) {
	in := []byte(`{"auth":{"profile":"bearer","secretRef":"/run/secrets/token"},"token":"abc"}`)
	got := RedactJSON(in)
	if bytes.Contains(got, []byte("/run/secrets/token")) || bytes.Contains(got, []byte("abc")) {
		t.Fatalf("leaked: %s", got)
	}
	if !bytes.Contains(got, []byte(Redacted)) {
		t.Fatalf("missing placeholder: %s", got)
	}
}

func TestRedactYAML(t *testing.T) {
	in := []byte("spec:\n  management:\n    auth:\n      secretRef: /run/secrets/token\n")
	got := RedactBytes(in)
	if bytes.Contains(got, []byte("/run/secrets/token")) {
		t.Fatalf("leaked: %s", got)
	}
}

func TestLooksLikeSecret(t *testing.T) {
	if !LooksLikeSecret("Bearer super-secret") {
		t.Fatal("bearer")
	}
	if LooksLikeSecret("add www A") {
		t.Fatal("plain")
	}
}
