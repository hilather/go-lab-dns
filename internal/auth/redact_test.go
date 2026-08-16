package auth

import (
	"bytes"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
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

func TestRedactOperationsAndState(t *testing.T) {
	ops := RedactOperations([]model.Operation{{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetManagement},
		Value:  []byte(`{"auth":{"profile":"bearer","secretRef":"/run/secrets/token"}}`),
	}})
	if bytes.Contains(ops[0].Value, []byte("/run/secrets/token")) {
		t.Fatalf("op leaked: %s", ops[0].Value)
	}
	st := &model.State{}
	st.Spec.Management.Auth.SecretRef = "/run/secrets/token"
	got := RedactState(st)
	if got.Spec.Management.Auth.SecretRef != Redacted {
		t.Fatalf("state secretRef=%q", got.Spec.Management.Auth.SecretRef)
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
