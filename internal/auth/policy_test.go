package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPolicyBearerRequiresTokens(t *testing.T) {
	if _, err := NewPolicy(PolicyConfig{Profile: ProfileBearer}); err == nil {
		t.Fatal("expected error")
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
}
