package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestLoadAcceptsOverlengthLabel(t *testing.T) {
	label := strings.Repeat("A", 64)
	if len(label) <= 63 {
		t.Fatal("test setup: label must exceed 63 octets")
	}
	doc := overlengthOverlayDoc("overlength-label-inline", "long-label-a", label, "10.42.0.64")
	st, err := Load([]byte(doc))
	if err != nil {
		t.Fatalf("Load rejected 64-octet label: %v", err)
	}
	got := st.Spec.Zones[0].Records[0].Owner
	want := strings.ToLower(label) + ".lab.example.net."
	if got != want {
		t.Fatalf("owner=%q want %q", got, want)
	}
	if !strings.HasSuffix(got, ".") {
		t.Fatal("stored owner must be an FQDN with a trailing dot")
	}
	if got != strings.ToLower(got) {
		t.Fatal("stored owner must be lower-case")
	}
}

func TestLoadAcceptsOverlengthPresentationFQDN(t *testing.T) {
	lab := strings.Repeat("B", 63)
	owner := strings.Join([]string{lab, lab, lab, lab, "Lab.Example.NET."}, ".")
	if len(owner) <= 254 {
		t.Fatalf("test setup: presentation name len=%d, want >254", len(owner))
	}
	doc := overlengthOverlayDoc("overlength-presentation-inline", "long-fqdn-a", owner, "10.42.0.65")
	st, err := Load([]byte(doc))
	if err != nil {
		t.Fatalf("Load rejected presentation FQDN of %d characters: %v", len(owner), err)
	}
	got := st.Spec.Zones[0].Records[0].Owner
	want := strings.ToLower(owner)
	if got != want {
		t.Fatalf("owner=%q want %q", got, want)
	}
	if !strings.HasSuffix(got, ".") {
		t.Fatal("stored owner must be an FQDN with a trailing dot")
	}
	if got != strings.ToLower(got) {
		t.Fatal("stored owner must be lower-case")
	}
}

func TestLoadFileOverlengthFixturesCanonicalizeOwners(t *testing.T) {
	cases := []struct {
		file      string
		wantOwner string
		check     func(t *testing.T, owner string)
	}{
		{
			file:      "overlength-label.yaml",
			wantOwner: strings.Repeat("a", 64) + ".lab.example.net.",
			check: func(t *testing.T, owner string) {
				t.Helper()
				lab := strings.Split(owner, ".")[0]
				if len(lab) < 64 {
					t.Fatalf("label %q len=%d, want >=64", lab, len(lab))
				}
			},
		},
		{
			file: "overlength-presentation.yaml",
			wantOwner: strings.ToLower(strings.Join([]string{
				strings.Repeat("x", 63),
				strings.Repeat("x", 63),
				strings.Repeat("x", 63),
				strings.Repeat("x", 63),
				"lab.example.net.",
			}, ".")),
			check: func(t *testing.T, owner string) {
				t.Helper()
				if len(owner) <= 254 {
					t.Fatalf("presentation FQDN len=%d, want >254", len(owner))
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			st, err := LoadFile(testdata(t, "valid", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			got := st.Spec.Zones[0].Records[0].Owner
			if got != tc.wantOwner {
				t.Fatalf("owner=%q want %q", got, tc.wantOwner)
			}
			tc.check(t, got)
		})
	}
}

func TestLoadRejectsNonLengthNameSyntax(t *testing.T) {
	cases := []struct {
		file string
		code string
	}{
		{"invalid-label-char.yaml", violationInvalidName},
		{"non-ascii.yaml", violationNonASCII},
		{"empty-label.yaml", violationInvalidName},
		{"empty-name.yaml", violationRequired},
		{"leading-hyphen.yaml", violationInvalidName},
		{"trailing-hyphen.yaml", violationInvalidName},
		{"non-leftmost-wildcard.yaml", violationInvalidName},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			_, err := LoadFile(testdata(t, "invalid", tc.file))
			_ = requireValidation(t, err, tc.code)
		})
	}
}

func TestOverlengthCanonicalJSONRoundTrip(t *testing.T) {
	for _, name := range []string{"overlength-label.yaml", "overlength-presentation.yaml"} {
		t.Run(name, func(t *testing.T) {
			st, err := LoadFile(testdata(t, "valid", name))
			if err != nil {
				t.Fatal(err)
			}
			want := st.Spec.Zones[0].Records[0].Owner
			raw, err := CanonicalJSON(st)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Load(raw)
			if err != nil {
				t.Fatal(err)
			}
			got := again.Spec.Zones[0].Records[0].Owner
			if got != want {
				t.Fatalf("round-trip owner=%q want %q", got, want)
			}
			rev1, err := Revision(st)
			if err != nil {
				t.Fatal(err)
			}
			rev2, err := Revision(again)
			if err != nil {
				t.Fatal(err)
			}
			if rev1 != rev2 {
				t.Fatalf("round-trip revision %s != %s", rev1, rev2)
			}
			if !strings.HasSuffix(got, ".") || got != strings.ToLower(got) {
				t.Fatalf("canonical owner %q", got)
			}
			if again.Spec.Zones[0].Name != model.Name("lab.example.net.") {
				t.Fatalf("zone name=%q", again.Spec.Zones[0].Name)
			}
		})
	}
}

func overlengthOverlayDoc(metaName, recID, owner, ipv4 string) string {
	return fmt.Sprintf(`apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: %s
spec:
  access:
    clientGroups: []
  zones:
    - id: lab-zone
      name: lab.example.net.
      mode: overlay
      records:
        - id: %s
          owner: %s
          type: A
          values: [%s]
`, metaName, recID, owner, ipv4)
}
