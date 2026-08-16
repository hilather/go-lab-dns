package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "labdns") {
		t.Fatalf("version output %q missing labdns", stdout.String())
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "chaos emergency-disable") {
		t.Fatalf("help %q missing chaos", stdout.String())
	}
}

func TestHelpMatchesGeneratedArtifact(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	path := filepath.Join(repoRoot(t), "api", "cli", "help.txt")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(want) {
		t.Fatalf("labdns help drifted from %s", path)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "not-a-command"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestServeRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "serve"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr %q missing --config", stderr.String())
	}
}

func TestValidateSuccessAndFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join(repoRoot(t), "testdata/config/valid/empty-client-groups.yaml")
	code := runContext(context.Background(), []string{"labdns", "validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok revision=sha256:") {
		t.Fatalf("stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	bad := filepath.Join(repoRoot(t), "testdata/config/invalid/unknown-field.yaml")
	code = runContext(context.Background(), []string{"labdns", "validate", "--config", bad}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("invalid exit %d want 1 stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"labdns", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("missing --config exit %d", code)
	}
}

func TestCanonicalizeYAMLAndJSON(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/valid/empty-client-groups.yaml")
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "canonicalize", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "apiVersion:") {
		t.Fatalf("yaml=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"labdns", "canonicalize", "--config", path, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("json exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"apiVersion"`) {
		t.Fatalf("json=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"labdns", "canonicalize", "--config", path, "--format", "xml"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("bad format exit %d", code)
	}
}

func TestVerifyProbes(t *testing.T) {
	cfg := filepath.Join(repoRoot(t), "testdata/config/valid/empty-client-groups.yaml")
	probes := filepath.Join(repoRoot(t), "testdata/container/probes.yaml")
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "verify", "--config", cfg, "--probes", probes}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"labdns", "verify", "--config", cfg}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("missing probes exit %d", code)
	}
}

func TestQueryAndHealthcheck(t *testing.T) {
	path := ephemeralPackSample(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt, err := serveFromConfig(ctx, serveFlags{Config: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })

	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{
		"labdns", "query", "--name", "ns1.lab.example.net.", "--type", "A", "--server", rt.UDPAddr().String(),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("query exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "10.42.0.53") {
		t.Fatalf("query=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	url := "http://" + rt.MgmtAddr() + "/v1/health/ready"
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	code = runContext(context.Background(), []string{"labdns", "healthcheck", "--url", url}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("healthcheck exit %d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"labdns", "healthcheck", "--url", "http://127.0.0.1:1/v1/health/ready"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("down healthcheck exit %d", code)
	}
}

func TestQueryRequiresName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "query"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestQueryRejectsUnknownTransport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "query", "--name", "ns1.lab.example.net.", "--transport", "dot"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--transport") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestHealthcheckUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "healthcheck", "--nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestChaosEmergencyDisableRequiresPIDFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "chaos", "emergency-disable"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--pid-file") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestChaosUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "chaos", "explode"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestChaosEmergencyDisableMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "chaos", "emergency-disable", "--pid-file", filepath.Join(t.TempDir(), "missing.pid")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
}

func TestReadWritePIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labdns.pid")
	if err := writePIDFile(path); err != nil {
		t.Fatal(err)
	}
	pid, err := readPIDFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid=%d want %d", pid, os.Getpid())
	}
}

func TestDockerfileHardening(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"FROM scratch",
		"USER 65532:65532",
		"Apache-2.0",
		"ghcr.io/hilather/labdns",
		`ENTRYPOINT ["/labdns"]`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Dockerfile missing %q", want)
		}
	}
	if strings.Contains(text, "unimplemented until PR-16") {
		t.Error("Dockerfile still the fail-closed stub")
	}
}
