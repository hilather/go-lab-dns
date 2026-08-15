package config

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestMigrateV1Alpha1Identity(t *testing.T) {
	raw := []byte(mustLoad(t, "valid", "empty-client-groups.yaml"))
	out, err := MigrateToCurrent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(raw) {
		t.Fatal("v1alpha1 migrate is not identity")
	}
	if PeekAPIVersion(raw) != model.APIVersionV1Alpha1 {
		t.Fatalf("peek=%q", PeekAPIVersion(raw))
	}
	if len(Migrations()) != 0 {
		t.Fatalf("expected no migrators, got %d", len(Migrations()))
	}
}

func TestMigrateUnknownVersion(t *testing.T) {
	raw := []byte("apiVersion: labdns.dev/v1beta1\nkind: LabDNS\nmetadata:\n  name: x\nspec: {}\n")
	_, err := MigrateToCurrent(raw)
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeUnsupportedProtocolVersion {
		t.Fatalf("got %#v", err)
	}
	if _, err := Load(raw); err == nil {
		t.Fatal("Load should reject unknown version")
	}
}

func TestMigrateJSONPeek(t *testing.T) {
	raw := []byte(`{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{}}`)
	if PeekAPIVersion(raw) != model.APIVersionV1Alpha1 {
		t.Fatalf("peek=%q", PeekAPIVersion(raw))
	}
}
