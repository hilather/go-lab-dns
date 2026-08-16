package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestStartupChaosDisableWinsOverYAML(t *testing.T) {
	path := writeChaosServfailConfig(t, "127.0.0.1:0", "127.0.0.1:0")
	ctx := context.Background()
	rt, err := serveFromConfig(ctx, serveFlags{Config: path, ChaosDisable: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	if live := rt.Store().Load(); live == nil || !live.EmergencyChaosOff {
		t.Fatalf("override did not set EmergencyChaosOff: %+v", live)
	}
	if !rt.Store().StartupChaosOff() {
		t.Fatal("startup lock not armed")
	}
	msg := queryA(t, rt, "ns1.lab.example.net.")
	if msg.RCode != model.RCodeNoError {
		t.Fatalf("override lost: rcode=%s (YAML would SERVFAIL)", msg.RCode)
	}
}

func TestStartupChaosDisableSurvivesResetAndEnable(t *testing.T) {
	path := writeChaosServfailConfig(t, "127.0.0.1:0", "127.0.0.1:0")
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: path, ChaosDisable: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	actor := auth.Actor{ID: "lifecycle", Class: "startup"}
	if _, err := rt.app.Reset(context.Background(), actor, app.ResetIn{Reason: "lifecycle"}); err != nil {
		t.Fatal(err)
	}
	if !rt.Store().StartupChaosOff() {
		t.Fatal("reset cleared startup lock")
	}
	if live := rt.Store().Load(); live == nil || !live.EmergencyChaosOff {
		t.Fatal("reset relaxed startup inhibit on the snapshot")
	}
	if msg := queryA(t, rt, "ns1.lab.example.net."); msg.RCode != model.RCodeNoError {
		t.Fatalf("after reset rcode=%s (startup override must still inhibit SERVFAIL)", msg.RCode)
	}
	if _, err := rt.app.EmergencyEnableChaos(context.Background(), actor, app.EmergencyIn{Reason: "resume"}); err != nil {
		t.Fatal(err)
	}
	if !rt.Store().StartupChaosOff() || !rt.Store().EmergencyChaosOff() {
		t.Fatal("emergency-enable relaxed startup inhibit")
	}
	if live := rt.Store().Load(); live == nil || !live.EmergencyChaosOff {
		t.Fatal("emergency-enable cleared snapshot inhibit under startup lock")
	}
	if msg := queryA(t, rt, "ns1.lab.example.net."); msg.RCode != model.RCodeNoError {
		t.Fatalf("after emergency-enable rcode=%s", msg.RCode)
	}
}

func TestStartupChaosDisableEnvWinsOverYAML(t *testing.T) {
	t.Setenv("LABDNS_CHAOS_DISABLE", "true")
	path := writeChaosServfailConfig(t, "127.0.0.1:0", "127.0.0.1:0")
	var stderr bytes.Buffer
	flags, err := parseServeFlags([]string{"--config", path}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !flags.ChaosDisable {
		t.Fatal("env override not parsed")
	}
	rt, err := serveFromConfig(context.Background(), flags)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	msg := queryA(t, rt, "ns1.lab.example.net.")
	if msg.RCode != model.RCodeNoError {
		t.Fatalf("env override lost: rcode=%s", msg.RCode)
	}
}

func TestSIGUSR1WorksWithManagementUnbound(t *testing.T) {
	bin := labdnsBin(t)
	dir := t.TempDir()
	cfg := writeChaosServfailConfig(t, "127.0.0.1:0", ":0")
	pidPath := filepath.Join(dir, "labdns.pid")
	cmd, stdout := startLabdns(t, bin, "serve",
		"--config", cfg,
		"--management-listen", "off",
		"--pid-file", pidPath,
	)
	line := waitOutput(t, stdout, "listening", 5*time.Second)
	if !strings.Contains(line, "management unbound") {
		t.Fatalf("expected unbound management, got %q", line)
	}
	udp := parseListenField(t, line, "udp")
	msg := queryAAddr(t, udp, "ns1.lab.example.net.")
	if msg.RCode != model.RCodeServFail {
		t.Fatalf("pre-signal rcode=%s want SERVFAIL", msg.RCode)
	}

	var out, errb bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "chaos", "emergency-disable", "--pid-file", pidPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("emergency-disable exit %d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "SIGUSR1") {
		t.Fatalf("stdout=%q", out.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg = queryAAddr(t, udp, "ns1.lab.example.net.")
		if msg.RCode == model.RCodeNoError {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	t.Fatalf("SIGUSR1 with management unbound did not disable chaos, last rcode=%s", msg.RCode)
}

func TestShutdownCancelsDelayedQueries(t *testing.T) {
	path := writeChaosDelayConfig(t, "127.0.0.1:0", "127.0.0.1:0", 2*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	rt, err := serveFromConfig(ctx, serveFlags{Config: path, ShutdownTimeout: time.Second})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = rt.Shutdown(context.Background())
	})

	q, err := dnswire.PackQuery(9, model.Query{
		Name: "ns1.lab.example.net.", Type: model.TypeA, Class: model.ClassIN, RD: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	go func() {
		close(started)
		_, _ = exchangeUDPTimeout(rt.UDPAddr().String(), q, 3*time.Second)
	}()
	<-started
	// Wait until the delay session is reserved so CancelDelays has a watcher.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && rt.engine.Stats().Delayed.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if rt.engine.Stats().Delayed.Load() == 0 {
		t.Fatal("delay never started")
	}
	start := time.Now()
	cancel()
	shctx, shcancel := context.WithTimeout(context.Background(), time.Second)
	defer shcancel()
	if err := rt.Shutdown(shctx); err != nil && err != context.DeadlineExceeded {
		t.Fatalf("shutdown: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 900*time.Millisecond {
		t.Fatalf("shutdown waited %s; delayed query was not canceled", elapsed)
	}
	if rt.engine.Stats().DelayCanceled.Load() < 1 {
		t.Fatal("shutdown did not cancel the in-flight delay")
	}
}

func TestRestartDiscardsRuntimeDrift(t *testing.T) {
	bin := labdnsBin(t)
	cfg := writeLocalConfig(t, "127.0.0.1:0", "127.0.0.1:0")
	cmd, stdout := startLabdns(t, bin, "serve", "--config", cfg)
	line := waitOutput(t, stdout, "listening", 5*time.Second)
	mgmt := "http://" + parseListenField(t, line, "management")
	st := getJSON(t, mgmt+"/v1/state")
	if drifted, _ := st["drifted"].(bool); drifted {
		t.Fatal("fresh process should not be drifted")
	}
	rev, _ := st["runtimeRevision"].(string)
	if rev == "" {
		t.Fatalf("state=%v", st)
	}
	applyBody := fmt.Sprintf(`{"expectedRevision":%q,"reason":"add www","operations":[{"op":"add","target":{"kind":"record","id":"www-a","zoneId":"lab-zone"},"value":{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}}]}`, rev)
	postJSON(t, mgmt+"/v1/changes:apply", applyBody)
	st = getJSON(t, mgmt+"/v1/state")
	if drifted, _ := st["drifted"].(bool); !drifted {
		t.Fatalf("expected drift after apply: %v", st)
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	waitCmd(t, cmd, 3*time.Second)

	cmd, stdout = startLabdns(t, bin, "serve", "--config", cfg)
	line = waitOutput(t, stdout, "listening", 5*time.Second)
	mgmt = "http://" + parseListenField(t, line, "management")
	st = getJSON(t, mgmt+"/v1/state")
	if drifted, _ := st["drifted"].(bool); drifted {
		t.Fatalf("restart kept drift: %v", st)
	}
	body, _ := json.Marshal(st["canonical"])
	if strings.Contains(string(body), "www-a") {
		t.Fatal("restart retained runtime record www-a")
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	waitCmd(t, cmd, 3*time.Second)
}

func TestChaosEmergencyDisableInvalidPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pid")
	if err := os.WriteFile(path, []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "chaos", "emergency-disable", "--pid-file", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
}

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

func labdnsBin(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "labdns-bin-")
		if err != nil {
			binErr = err
			return
		}
		binPath = filepath.Join(dir, "labdns")
		cmd := exec.Command("go", "build", "-o", binPath, ".")
		cmd.Dir = filepath.Join(repoRoot(t), "cmd", "labdns")
		out, err := cmd.CombinedOutput()
		if err != nil {
			binErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if binErr != nil {
		t.Fatal(binErr)
	}
	return binPath
}

func startLabdns(t *testing.T, bin string, args ...string) (*exec.Cmd, *syncBuf) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	stdout := &syncBuf{}
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				_ = cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	})
	return cmd, stdout
}

func waitOutput(t *testing.T, buf *syncBuf, needle string, d time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		s := buf.String()
		if strings.Contains(s, needle) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q, output=%q", needle, buf.String())
	return ""
}

func waitCmd(t *testing.T, cmd *exec.Cmd, d time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(d):
		_ = cmd.Process.Kill()
		t.Fatalf("process did not exit in %s", d)
	}
}

func parseListenField(t *testing.T, line, field string) string {
	t.Helper()
	// "labdns: listening udp A tcp B management C revision D"
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == field && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	t.Fatalf("missing %s in %q", field, line)
	return ""
}

func queryA(t *testing.T, rt *serveRuntime, name string) *dnswire.UpstreamMsg {
	t.Helper()
	return queryAAddr(t, rt.UDPAddr().String(), name)
}

func queryAAddr(t *testing.T, addr, name string) *dnswire.UpstreamMsg {
	t.Helper()
	q, err := dnswire.PackQuery(3, model.Query{
		Name: model.Name(name), Type: model.TypeA, Class: model.ClassIN, RD: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := exchangeUDPTimeout(addr, q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := dnswire.UnpackUpstream(raw)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func exchangeUDPTimeout(addr string, payload []byte, d time.Duration) ([]byte, error) {
	c, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(d))
	if _, err := c.Write(payload); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	var last error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			last = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		defer func() { _ = resp.Body.Close() }()
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	t.Fatalf("GET %s: %v", url, last)
	return nil
}

func postJSON(t *testing.T, url, body string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s status=%d body=%s", url, resp.StatusCode, b)
	}
}

func writeLocalConfig(t *testing.T, dnsAddr, mgmtAddr string) string {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: local
spec:
  listeners:
    dns:
      address: %q
      protocols: [udp, tcp]
    management:
      address: %q
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
      nameservers: [ns1.lab.example.net.]
      records:
        - id: ns1-a
          owner: ns1
          type: A
          values: [10.42.0.53]
`, dnsAddr, mgmtAddr)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeChaosServfailConfig(t *testing.T, dnsAddr, mgmtAddr string) string {
	t.Helper()
	return writeChaosConfig(t, dnsAddr, mgmtAddr, `
        outcomes:
          - id: fail
            weight: 100
            actions:
              - type: rcode
                value: SERVFAIL
`)
}

func writeChaosDelayConfig(t *testing.T, dnsAddr, mgmtAddr string, d time.Duration) string {
	t.Helper()
	return writeChaosConfig(t, dnsAddr, mgmtAddr, fmt.Sprintf(`
        outcomes:
          - id: delayed
            weight: 100
            actions:
              - type: delay
                phase: before-response
                distribution: fixed
                duration: %s
`, d))
}

func writeChaosConfig(t *testing.T, dnsAddr, mgmtAddr, outcomes string) string {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: chaos-lab
spec:
  listeners:
    dns:
      address: %q
      protocols: [udp, tcp]
    management:
      address: %q
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
      nameservers: [ns1.lab.example.net.]
      records:
        - id: ns1-a
          owner: ns1
          type: A
          values: [10.42.0.53]
          chaosPolicyRefs: [always]
  chaos:
    enabled: true
    safety:
      maxDelay: 10s
      maxConcurrentDelayed: 64
      maxDropProbability: 1.0
    policies:
      - id: always
        owner: testers
        reason: lifecycle test
        enabled: true
        safetyClass: low
        scope:
          recordIds: [ns1-a]
        selector:
          mode: deterministic
          seed: lifecycle
          probability: 1.0
%s
`, dnsAddr, mgmtAddr, outcomes)
	path := filepath.Join(t.TempDir(), "chaos.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
