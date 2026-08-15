package config

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestRevisionStableAcrossFormatting(t *testing.T) {
	compact := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: primary-lab
spec:
  access:
    clientGroups: []
  defaults:
    ttl: 30s
    negativeTTL: 10s
  zones:
    - id: lab-zone
      name: lab.example.net.
      mode: authoritative
      soa:
        primary: ns1.lab.example.net.
        administrator: hostmaster.lab.example.net.
        serial: auto
        refresh: 1h
        retry: 5m
        expire: 24h
      records:
        - id: ns1-a
          owner: ns1
          type: A
          values: [10.42.0.53]
`
	spaced := strings.ReplaceAll(compact, "\n", "\n\n")
	spaced = "# header comment\n" + spaced
	a, err := Load([]byte(compact))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load([]byte(spaced))
	if err != nil {
		t.Fatal(err)
	}
	ra, err := Revision(a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := Revision(b)
	if err != nil {
		t.Fatal(err)
	}
	if ra != rb {
		t.Fatalf("formatting changed revision\n%s\n%s", ra, rb)
	}
	if !strings.HasPrefix(string(ra), model.RevisionPrefix) {
		t.Fatalf("revision %q missing prefix", ra)
	}
	if len(ra) != len(model.RevisionPrefix)+64 {
		t.Fatalf("revision len=%d", len(ra))
	}
}

func TestRevisionChangesOnSemanticEdit(t *testing.T) {
	a, err := Load([]byte(mustLoad(t, "valid", "empty-client-groups.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	ra, err := Revision(a)
	if err != nil {
		t.Fatal(err)
	}
	a.Spec.Zones[0].Records[0].Values[0] = "10.42.0.99"
	rb, err := Revision(a)
	if err != nil {
		t.Fatal(err)
	}
	if ra == rb {
		t.Fatal("semantic change did not change revision")
	}
}

func TestCanonicalExportMaterializesDefaults(t *testing.T) {
	st, err := Load([]byte(mustLoad(t, "valid", "empty-client-groups.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CanonicalJSON(st)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		`"unknownClient":"refuse-forward"`,
		`"cnameDepth":8`,
		`":5353"`,
		`":8080"`,
		`"/v1"`,
		`"/mcp"`,
		`"30s"`,
		`"10s"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("canonical JSON missing %s\n%s", want, s)
		}
	}
	y, err := CanonicalYAML(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(y), "#") {
		t.Fatalf("YAML export contains comment:\n%s", y)
	}
}

func TestDuration30000msSameAs30s(t *testing.T) {
	a := `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: x
spec:
  defaults:
    ttl: 30s
    negativeTTL: 10s
  zones:
    - id: z
      name: lab.example.net.
      mode: overlay
      records:
        - id: r
          owner: a
          type: A
          ttl: 30s
          values: [10.0.0.1]
`
	b := strings.ReplaceAll(a, "ttl: 30s", "ttl: 30000ms")
	sa, err := Load([]byte(a))
	if err != nil {
		t.Fatal(err)
	}
	sb, err := Load([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	ra, _ := Revision(sa)
	rb, _ := Revision(sb)
	if ra != rb {
		t.Fatalf("30s vs 30000ms changed revision %s %s", ra, rb)
	}
}
