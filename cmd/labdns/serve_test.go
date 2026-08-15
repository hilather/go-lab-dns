package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestServeFromConfigPackSampleNS1(t *testing.T) {
	path := ephemeralPackSample(t)
	ctx := testutil.Context(t)
	srv, snap, err := serveFromConfig(ctx, serveFlags{Config: path})
	if err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() {
		_ = srv.Shutdown(t.Context())
	})
	if snap == nil || snap.Revision == "" {
		t.Fatal("missing compiled snapshot")
	}
	if srv.UDPAddr() == nil {
		t.Fatal("UDP not bound")
	}

	q, err := dnswire.PackQuery(7, model.Query{
		Name: "ns1.lab.example.net.", Type: model.TypeA, Class: model.ClassIN, RD: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := exchangeUDP(t, srv.UDPAddr(), q)
	msg, err := dnswire.UnpackUpstream(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode != model.RCodeNoError || !msg.AA || msg.RA {
		t.Fatalf("flags rcode=%s AA=%v RA=%v", msg.RCode, msg.AA, msg.RA)
	}
	found := false
	for _, rr := range msg.Answers {
		if rr.Type == model.TypeA && rr.Data == "10.42.0.53" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing ns1 A: %+v", msg.Answers)
	}
}

func TestServeFromConfigInvalidDoesNotListen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	body := "apiVersion: labdns.dev/v1alpha1\nkind: LabDNS\nmetadata:\n  name: bad\nspec:\n  listeners:\n    dns:\n      address: " + addr + "\n  notAField: true\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, snap, err := serveFromConfig(context.Background(), serveFlags{Config: path})
	if err == nil || srv != nil || snap != nil {
		t.Fatalf("invalid bootstrap bound: srv=%v snap=%v err=%v", srv, snap, err)
	}

	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("configured listen address still bound after invalid bootstrap: %v", err)
	}
	testutil.MustClose(t, ln2)
}

func TestServeCLIMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "serve"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2, stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "listening") {
		t.Fatalf("stdout leaked listen: %q", stdout.String())
	}
}

func TestServeCLIInvalidConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join(repoRoot(t), "testdata/config/invalid/unknown-field.yaml")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	code := runContext(ctx, []string{"labdns", "serve", "--config", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1, stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "listening") {
		t.Fatalf("invalid bootstrap listened: %q", stdout.String())
	}
}

func TestServeCLIShutdown(t *testing.T) {
	path := ephemeralPackSample(t)
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr syncBuf
	done := make(chan int, 1)
	go func() {
		done <- runContext(ctx, []string{"labdns", "serve", "--config", path}, &stdout, &stderr)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stdout.String(), "listening") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(stdout.String(), "listening") {
		cancel()
		t.Fatalf("serve never listened, stderr=%q stdout=%q", stderr.String(), stdout.String())
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d, stderr=%q", code, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not shut down after cancel")
	}
	if !strings.Contains(stdout.String(), "shutting down") {
		t.Fatalf("stdout %q missing shutdown", stdout.String())
	}
}

func TestParseServeFlagsChaosDisable(t *testing.T) {
	var stderr bytes.Buffer
	f, err := parseServeFlags([]string{"--config", "/tmp/x.yaml", "--chaos-disable"}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !f.ChaosDisable || f.Config != "/tmp/x.yaml" {
		t.Fatalf("%+v", f)
	}
}

func TestDnsListenAddrsDefault(t *testing.T) {
	udp, tcp := dnsListenAddrs(nil)
	if udp != ":5353" || tcp != ":5353" {
		t.Fatalf("udp=%q tcp=%q", udp, tcp)
	}
}

func ephemeralPackSample(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata/config/valid/pack-sample.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(string(src), `address: ":5353"`, `address: "127.0.0.1:0"`, 1)
	if rewritten == string(src) {
		t.Fatal("pack-sample listen address not rewritten")
	}
	path := filepath.Join(t.TempDir(), "pack-sample.yaml")
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func exchangeUDP(t *testing.T, addr net.Addr, payload []byte) []byte {
	t.Helper()
	c, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	testutil.MustClose(t, c)
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf[:n]
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

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
