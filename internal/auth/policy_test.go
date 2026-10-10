package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const wantBearerNoToken = "validation_failed: bearer profile has no usable token; set spec.management.auth.secretRef to a token file, or set profile: dev-loopback-unauth"

func TestPolicyBearerRequiresTokens(t *testing.T) {
	_, err := NewPolicy(PolicyConfig{Profile: ProfileBearer})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != wantBearerNoToken {
		t.Fatalf("err=%q", err.Error())
	}
}

func TestNewPolicyEmptyProfileIsBearer(t *testing.T) {
	p, err := NewPolicy(PolicyConfig{Tokens: []Token{{Token: "t", Role: RoleAdministrator}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Profile() != ProfileBearer {
		t.Fatalf("profile=%s", p.Profile())
	}
}

func TestLoadTokensPlainAndJSON(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "tok")
	if err := os.WriteFile(plain, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := NewPolicy(PolicyConfig{Profile: ProfileBearer, SecretRef: plain})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Authenticate(context.Background(), "s3cret")
	if err != nil || a.Role != RoleAdministrator {
		t.Fatalf("%+v %v", a, err)
	}

	js := filepath.Join(dir, "tok.json")
	body := `{"tokens":[{"token":"v1","id":"alice","role":"viewer"}]}`
	if err := os.WriteFile(js, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p2, err := NewPolicy(PolicyConfig{Profile: ProfileBearer, SecretRef: js})
	if err != nil {
		t.Fatal(err)
	}
	a2, err := p2.Authenticate(context.Background(), "v1")
	if err != nil || a2.ID != "alice" || a2.Role != RoleViewer {
		t.Fatalf("%+v %v", a2, err)
	}
}

func TestPolicyMissingSecretFailsClosed(t *testing.T) {
	_, err := NewPolicy(PolicyConfig{Profile: ProfileBearer, SecretRef: "/no/such/file"})
	if err == nil {
		t.Fatal("expected fail closed")
	}
	if err.Error() != wantBearerNoToken {
		t.Fatalf("err=%q", err.Error())
	}
}

func TestPolicyInvalidJSONStaysInvalidTokenSecret(t *testing.T) {
	dir := t.TempDir()
	for _, body := range []string{`{"tokens":[]}`, `[]`, `{`} {
		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := NewPolicy(PolicyConfig{Profile: ProfileBearer, SecretRef: path})
		if err == nil || err.Error() != "validation_failed: invalid token secret" {
			t.Fatalf("body %s err=%v", body, err)
		}
	}
}

func TestPolicyDevLoopbackKeepsLoadTokensText(t *testing.T) {
	_, err := NewPolicy(PolicyConfig{Profile: ProfileDevLoopbackUnauth, SecretRef: "/no/such/file"})
	if err == nil || err.Error() != "unauthenticated: token secret is unavailable" {
		t.Fatalf("err=%v", err)
	}
	_, err = NewPolicy(PolicyConfig{Profile: ProfileDevLoopbackUnauth, Tokens: []Token{{Token: ""}}})
	if err == nil || err.Error() != "validation_failed: empty token" {
		t.Fatalf("empty value err=%v", err)
	}
}
