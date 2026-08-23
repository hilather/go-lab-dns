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
	_ = requireValidation(t, err, violationUnknownField)
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
		{"ui", `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{"ui":{"theme":"dark"}}}`, "spec.ui.theme"},
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

func TestDecodeOmittedUIEnabledTrueWithoutAccess(t *testing.T) {
	doc := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: x
spec: {}
`
	st, err := Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("omitted spec.ui with no access block must materialize enabled true")
	}
}

func TestDecodeUIEnabledExplicitFalse(t *testing.T) {
	doc := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: x
spec:
  ui:
    enabled: false
`
	st, err := Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.UI.Enabled {
		t.Fatal("explicit spec.ui.enabled: false was overwritten")
	}
}

func TestDecodeUIEnabledOmittedOnEmptyUIObject(t *testing.T) {
	doc := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: x
spec:
  ui: {}
`
	st, err := Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("omitted enabled on spec.ui must materialize true")
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

func TestDecodeRejectsTrailingDocuments(t *testing.T) {
	yamlDoc := "apiVersion: labdns.dev/v1alpha1\nkind: LabDNS\nmetadata:\n  name: a\nspec: {}\n---\nkind: LabDNS\n"
	_, err := DecodeYAML([]byte(yamlDoc))
	_ = requireValidation(t, err, violationInvalidValue)

	jsonDoc := `{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"a"},"spec":{}}{"x":1}`
	_, err = DecodeJSON([]byte(jsonDoc))
	_ = requireValidation(t, err, violationInvalidValue)
}

func TestDecodeRejectsUnitlessDuration(t *testing.T) {
	_, err := Decode([]byte("apiVersion: labdns.dev/v1alpha1\nkind: LabDNS\nmetadata:\n  name: x\nspec:\n  defaults:\n    ttl: 30\n"))
	_ = requireValidation(t, err, violationInvalidValue)
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

func TestLoadUIDisabledPreservesFalse(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "ui-disabled.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.UI.Enabled {
		t.Fatal("ui-disabled.yaml materialized enabled true")
	}
	if len(st.Spec.Management.AllowedOrigins) != 1 || st.Spec.Management.AllowedOrigins[0] != "https://dns-mgmt.lab.example" {
		t.Fatalf("allowedOrigins=%v", st.Spec.Management.AllowedOrigins)
	}
}

func TestLoadMCPAllowLegacyClients(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "mcp-legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Management.MCP == nil || !st.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("mcp-legacy.yaml did not decode allowLegacyClients true")
	}
	omitted, err := LoadFile(testdata(t, "valid", "ui-disabled.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if omitted.Spec.Management.MCP != nil && omitted.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("omitted mcp.allowLegacyClients must stay false")
	}
}

func TestLoadOmitUINoAccessEnabled(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "omit-ui-no-access.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("omit-ui-no-access.yaml did not materialize enabled true")
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
