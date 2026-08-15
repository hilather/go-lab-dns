package config

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestDecodePackSampleYAML(t *testing.T) {
	st, err := Decode([]byte(mustLoad(t, "valid", "pack-sample.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if st.APIVersion != model.APIVersionV1Alpha1 || st.Kind != model.KindLabDNS {
		t.Fatalf("api=%q kind=%q", st.APIVersion, st.Kind)
	}
	if st.Metadata.Name != "primary-lab" {
		t.Fatalf("name=%q", st.Metadata.Name)
	}
	if len(st.Spec.Access.ClientGroups) != 2 {
		t.Fatalf("groups=%d", len(st.Spec.Access.ClientGroups))
	}
	for _, g := range st.Spec.Access.ClientGroups {
		if !g.AllowForward {
			t.Fatalf("group %s allowForward=false after decode; omitted key must materialize true", g.ID)
		}
	}
}

func TestDecodeJSONUnknownField(t *testing.T) {
	_, err := DecodeJSON([]byte(`{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"nope":1}}`))
	requireValidation(t, err, violationUnknownField)
}

func TestDecodeUnknownFieldEveryLevel(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		path string
	}{
		{"root", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"extra":true,"spec":{}}`, "extra"},
		{"metadata", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x","nope":1},"spec":{}}`, "metadata.nope"},
		{"spec", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"nope":1}}`, "spec.nope"},
		{"defaults", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"defaults":{"ttl":"30s","zzz":1}}}`, "spec.defaults.zzz"},
		{"access", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"access":{"foo":1}}}`, "spec.access.foo"},
		{"clientGroup", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"access":{"clientGroups":[{"id":"g","cidrs":["10.0.0.0/8"],"x":true}]}}}`, "spec.access.clientGroups[0].x"},
		{"zone", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"zones":[{"id":"z","name":"lab.example.net.","mode":"overlay","weird":1,"records":[]}]}}`, "spec.zones[0].weird"},
		{"record", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"zones":[{"id":"z","name":"lab.example.net.","mode":"overlay","records":[{"id":"r","owner":"a","type":"A","values":["10.0.0.1"],"zzz":1}]}]}}`, "spec.zones[0].records[0].zzz"},
		{"chaos", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"chaos":{"nope":true}}}`, "spec.chaos.nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tc.doc))
			de := requireValidation(t, err, violationUnknownField)
			found := false
			for _, v := range de.FieldViolations {
				if v.Path == tc.path {
					found = true
				}
			}
			if !found {
				t.Fatalf("want path %q in %+v", tc.path, de.FieldViolations)
			}
		})
	}
}

func TestDecodeYAMLCommentsDropped(t *testing.T) {
	doc := "apiVersion: labdns.dev/v1alpha1\nkind: LabDNS\n# keep me\nmetadata:\n  name: x\nspec: {}\n"
	st, err := DecodeYAML([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	y, err := CanonicalYAML(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(y), "keep me") {
		t.Fatalf("comment preserved:\n%s", y)
	}
}

func TestDecodeAllowForwardExplicitFalse(t *testing.T) {
	doc := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: x
spec:
  access:
    clientGroups:
      - id: local
        cidrs: [10.0.0.0/8]
        allowForward: false
`
	st, err := Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Access.ClientGroups[0].AllowForward {
		t.Fatal("explicit allowForward: false was overwritten")
	}
}

func TestDecodeRejectsEmptyAndTooLarge(t *testing.T) {
	if _, err := Decode(nil); err == nil {
		t.Fatal("empty")
	}
	big := make([]byte, MaxDocumentBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	if _, err := Decode(big); err == nil {
		t.Fatal("too large")
	}
}

func TestDecodeJSONRoundTripSample(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "pack-sample.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CanonicalJSON(st)
	if err != nil {
		t.Fatal(err)
	}
	st2, err := Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	r1, err := Revision(st)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Revision(st2)
	if err != nil {
		t.Fatal(err)
	}
	if r1 != r2 {
		t.Fatalf("export/reimport revision %s != %s", r1, r2)
	}
}
