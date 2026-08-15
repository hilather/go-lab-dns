package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// expectedInvalid is the fail-closed code each negative fixture must produce.
var expectedInvalid = map[string]string{
	"time-bucket.yaml":          violationTimeBucket,
	"unknown-field.yaml":        violationUnknownField,
	"unknown-field-nested.yaml": violationUnknownField,
	"non-ascii.yaml":            violationNonASCII,
	"cname-coexist.yaml":        violationCNAMECoexist,
	"cname-loop.yaml":           violationCNAMELoop,
	"wildcard-ns.yaml":          violationWildcardNS,
	"wildcard-dname.yaml":       violationWildcardDNAME,
	"missing-pool.yaml":         violationUnresolved,
	"missing-chaos-ref.yaml":    violationUnresolved,
	"self-forward.yaml":         violationForwardLoop,
	"transport-dot.yaml":        violationInvalidTransport,
	"duplicate-id.yaml":         violationDuplicateID,
	"empty-id.yaml":             violationEmptyID,
	"unknown-client.yaml":       violationInvalidValue,
	"ttl-unitless.yaml":         violationInvalidValue,
	"invalid-label-char.yaml":   violationInvalidName,
	"multi-doc.yaml":            violationInvalidValue,
	"protected-record.yaml":     violationProtected,
	"duplicate-zone-name.yaml":  violationDuplicateID,
	"transport-udp.yaml":        violationInvalidTransport,
}

// TestConfigCompat is the positive+negative fixture matrix for make test-config-compat.
func TestConfigCompat(t *testing.T) {
	validDir := testdata(t, "valid")
	ents, err := os.ReadDir(validDir)
	if err != nil {
		t.Fatal(err)
	}
	var validCount int
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		validCount++
		name := e.Name()
		t.Run("valid/"+name, func(t *testing.T) {
			st, err := LoadFile(filepath.Join(validDir, name))
			if err != nil {
				t.Fatal(err)
			}
			if st.Spec.Access.UnknownClient != model.UnknownClientRefuseForward {
				t.Fatalf("unknownClient=%q", st.Spec.Access.UnknownClient)
			}
			rev, err := Revision(st)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := CanonicalJSON(st)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Load(raw)
			if err != nil {
				t.Fatal(err)
			}
			rev2, err := Revision(again)
			if err != nil {
				t.Fatal(err)
			}
			if rev != rev2 {
				t.Fatalf("round-trip revision %s != %s", rev, rev2)
			}
			assertCanonicalMatchesSchema(t, raw)
		})
	}
	if validCount < 2 {
		t.Fatalf("expected at least pack-sample and empty-client-groups, got %d", validCount)
	}

	invalidDir := testdata(t, "invalid")
	ients, err := os.ReadDir(invalidDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range ients {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := e.Name()
		want, ok := expectedInvalid[name]
		if !ok {
			t.Fatalf("no expectedInvalid code for %s (fail-closed: add it)", name)
		}
		seen[name] = true
		t.Run("invalid/"+name, func(t *testing.T) {
			_, err := LoadFile(filepath.Join(invalidDir, name))
			de := requireValidation(t, err, want)
			if de.Code != domainerr.CodeValidationFailed && de.Code != domainerr.CodeUnsupportedProtocolVersion {
				t.Fatalf("code=%s", de.Code)
			}
		})
	}
	for name := range expectedInvalid {
		if !seen[name] {
			t.Fatalf("missing invalid fixture %s", name)
		}
	}
}

func TestGoldenCanonicalJSON(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "empty-client-groups.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CanonicalJSON(st)
	if err != nil {
		t.Fatal(err)
	}
	rev, err := Revision(st)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := testdata(t, "golden", "empty-client-groups.json")
	revPath := testdata(t, "golden", "empty-client-groups.revision")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if mkErr := os.MkdirAll(filepath.Dir(goldenPath), 0o755); mkErr != nil {
			t.Fatal(mkErr)
		}
		if werr := os.WriteFile(goldenPath, append(raw, '\n'), 0o644); werr != nil {
			t.Fatal(werr)
		}
		if werr := os.WriteFile(revPath, []byte(string(rev)+"\n"), 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(want)) != string(raw) {
		t.Fatalf("golden mismatch\n got=%s\nwant=%s", raw, want)
	}
	wantRev, err := os.ReadFile(revPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(wantRev)) != string(rev) {
		t.Fatalf("revision golden %q got %q", strings.TrimSpace(string(wantRev)), rev)
	}
}
