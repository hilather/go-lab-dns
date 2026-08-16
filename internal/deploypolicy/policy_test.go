package deploypolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestCheckImage(t *testing.T) {
	ok := "ghcr.io/hilather/labdns@sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if err := CheckImage(ok); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"",
		"ghcr.io/hilather/labdns:latest",
		"ghcr.io/hilather/labdns:v1.0.0",
		"ghcr.io/hilather/labdns",
		"ghcr.io/hilather/labdns@sha256:dead",
		"ghcr.io/hilather/labdns@sha256:REPLACE_WITH_DIGEST",
	} {
		if err := CheckImage(bad); err == nil {
			t.Fatalf("accepted unpinned %q", bad)
		}
	}
}

func TestParseImageEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.env")
	body := "# comment\nLABDNS_IMAGE=ghcr.io/hilather/labdns@sha256:" + strings.Repeat("ab", 32) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ParseImageEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckImage(got); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("OTHER=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseImageEnv(path); err == nil {
		t.Fatal("missing LABDNS_IMAGE")
	}
}

func TestCheckRejectsBroadeningAndUnsafeChaos(t *testing.T) {
	dir := t.TempDir()
	writePolicyTree(t, dir)
	pol, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := sampleState(t)
	if err := Check(st, pol); err != nil {
		t.Fatalf("valid sample: %v", err)
	}

	st.Spec.Access.ClientGroups[0].CIDRs = []string{"0.0.0.0/0"}
	if err := Check(st, pol); err == nil || !strings.Contains(err.Error(), "allowed-client-networks") {
		t.Fatalf("broad CIDR: %v", err)
	}
	st = sampleState(t)
	st.Spec.Forwarding.Pools[0].Upstreams[0].Endpoint = "8.8.8.8:53"
	if err := Check(st, pol); err == nil || !strings.Contains(err.Error(), "allowed-upstreams") {
		t.Fatalf("unapproved upstream: %v", err)
	}
	st = sampleState(t)
	st.Spec.Chaos.Safety.MaxDelay = 0
	st.Spec.Chaos.Safety.MaxDelay = sampleState(t).Spec.Chaos.Safety.MaxDelay * 10
	if st.Spec.Chaos.Safety.MaxDelay == 0 {
		st.Spec.Chaos.Safety.MaxDelay = pol.MaxDelay * 2
	}
	if err := Check(st, pol); err == nil || !strings.Contains(err.Error(), "maxDelay") {
		t.Fatalf("unsafe chaos: %v", err)
	}
	st = sampleState(t)
	st.Spec.Chaos.Safety.ProtectedNames = nil
	if err := Check(st, pol); err == nil || !strings.Contains(err.Error(), "protected name") {
		t.Fatalf("missing protected: %v", err)
	}
}

func TestLoadDirRequiresAllKinds(t *testing.T) {
	dir := t.TempDir()
	writePolicyTree(t, dir)
	if _, err := LoadDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "allowed-client-networks.yaml")); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "AllowedClientNetworks") {
		t.Fatalf("missing kind: %v", err)
	}
}

func TestCheckKustomizeImageMatchesPin(t *testing.T) {
	dir := t.TempDir()
	pin := "ghcr.io/hilather/labdns@sha256:" + strings.Repeat("ab", 32)
	path := filepath.Join(dir, "kustomization.yaml")
	body := "images:\n  - name: ghcr.io/hilather/labdns\n    digest: sha256:" + strings.Repeat("ab", 32) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckKustomizeImage(path, pin); err != nil {
		t.Fatal(err)
	}
	mismatch := "images:\n  - name: ghcr.io/hilather/labdns\n    digest: sha256:" + strings.Repeat("00", 32) + "\n"
	if err := os.WriteFile(path, []byte(mismatch), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckKustomizeImage(path, pin); err == nil {
		t.Fatal("mismatched digest")
	}
	tagged := "images:\n  - name: ghcr.io/hilather/labdns\n    newTag: latest\n"
	if err := os.WriteFile(path, []byte(tagged), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckKustomizeImage(path, pin); err == nil {
		t.Fatal("newTag")
	}
}

func TestLoadDirRejectsUnknownKind(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "odd.yaml"), []byte("apiVersion: labdns.dev/policy/v1alpha1\nkind: NotAPolicy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("unknown kind")
	}
}

func writePolicyTree(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"allowed-upstreams.yaml": `apiVersion: labdns.dev/policy/v1alpha1
kind: AllowedUpstreams
endpoints: ["10.0.0.53:53", "10.0.0.54:53", "10.0.0.55:53"]
`,
		"allowed-client-networks.yaml": `apiVersion: labdns.dev/policy/v1alpha1
kind: AllowedClientNetworks
networks: ["10.42.0.0/16"]
`,
		"allowed-alternate-addresses.yaml": `apiVersion: labdns.dev/policy/v1alpha1
kind: AllowedAlternateAddresses
cidrs: ["10.42.0.0/16"]
`,
		"protected-names.yaml": `apiVersion: labdns.dev/policy/v1alpha1
kind: ProtectedNames
names: ["dns.lab.example.net."]
`,
		"chaos-safety.yaml": `apiVersion: labdns.dev/policy/v1alpha1
kind: ChaosSafety
maxDelay: 10s
maxConcurrentDelayed: 2000
maxDropProbability: 0.5
maxActiveHighImpactPolicies: 1
requireProtectedNames: ["dns.lab.example.net."]
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func sampleState(t *testing.T) *model.State {
	t.Helper()
	root := repoRoot(t)
	st, err := config.LoadFile(filepath.Join(root, "testdata", "config", "valid", "pack-sample.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
