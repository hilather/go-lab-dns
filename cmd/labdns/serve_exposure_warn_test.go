package main

import (
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/auth"
)

// TestServeExposureWarn drives serve and checks the stdout warning.
// The sentence is hard-coded here so a production text edit fails the test.
func TestServeExposureWarn(t *testing.T) {
	const dns = "127.0.0.1:0"
	tokenPath := writeExposureBearerToken(t)

	cases := []struct {
		name       string
		mgmt       string
		profile    string
		secretRef  string
		flag       string
		warn       bool
		quiet      bool
		unbound    bool
		localhost  bool
		skipListen bool
		fail       bool
	}{
		{name: "omitted 0.0.0.0:0", mgmt: "0.0.0.0:0", warn: true},
		{name: "omitted :0", mgmt: ":0", warn: true},
		{name: "explicit dev-loopback-unauth :0", mgmt: ":0", profile: "dev-loopback-unauth", warn: true},
		{name: "flag wildcard overrides loopback yaml", mgmt: "127.0.0.1:0", flag: "0.0.0.0:0", warn: true},
		{name: "omitted ipv4-mapped wildcard", mgmt: "[::ffff:0.0.0.0]:0", warn: true, skipListen: true},

		{name: "omitted 127.0.0.1:0", mgmt: "127.0.0.1:0", quiet: true},
		{name: "omitted 127.0.0.2:0", mgmt: "127.0.0.2:0", quiet: true},
		{name: "omitted ::1", mgmt: "[::1]:0", quiet: true, skipListen: true},
		{name: "omitted localhost", mgmt: "localhost:0", quiet: true, localhost: true},
		{name: "omitted ipv4-mapped loopback", mgmt: "[::ffff:127.0.0.1]:0", quiet: true, skipListen: true},
		{name: "flag loopback overrides wildcard yaml", mgmt: "0.0.0.0:0", flag: "127.0.0.1:0", quiet: true},
		{name: "bearer :0", mgmt: ":0", profile: "bearer", secretRef: tokenPath, quiet: true},
		{name: "management off", mgmt: "0.0.0.0:0", flag: "off", quiet: true, unbound: true},
		{name: "management OFF", mgmt: "0.0.0.0:0", flag: "OFF", quiet: true, unbound: true},

		{name: "test-net", mgmt: "127.0.0.1:0", flag: "192.0.2.10:0", fail: true},
		{name: "bad host", mgmt: "127.0.0.1:0", flag: "no-such-labdns-host.invalid:0", fail: true},
		{name: "missing port", mgmt: "127.0.0.1:0", flag: "127.0.0.1", fail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			if tc.profile == "" {
				path = writeLocalConfig(t, dns, tc.mgmt)
			} else {
				path = writeLocalConfigAuth(t, dns, tc.mgmt, tc.profile, tc.secretRef)
			}
			var extra []string
			if tc.flag != "" {
				extra = []string{"--management-listen=" + tc.flag}
			}
			stdout, stderr, code := runServeExposure(t, path, extra...)
			if tc.skipListen && code != 0 && strings.Contains(stderr, "management listen") {
				t.Skipf("cannot listen on %s: %s", tc.mgmt, strings.TrimSpace(stderr))
			}
			switch {
			case tc.fail:
				assertExposureFail(t, stdout, stderr, code)
			case tc.warn:
				assertExposureWarn(t, stdout, stderr, code)
			case tc.quiet:
				assertExposureQuiet(t, stdout, stderr, code)
				line := exposureListenLine(t, stdout)
				if tc.unbound && !strings.Contains(line, "management unbound") {
					t.Fatalf("listening line %q", line)
				}
				if tc.localhost {
					mgmt := parseListenField(t, line, "management")
					if !strings.HasPrefix(mgmt, "127.0.0.1:") && !strings.HasPrefix(mgmt, "[::1]:") {
						t.Fatalf("localhost bound %s", mgmt)
					}
				}
			default:
				t.Fatal("case has no expectation")
			}
		})
	}
}

// writeExposureBearerToken writes a 32-byte hex token (openssl rand -hex 32 shape).
func writeExposureBearerToken(t *testing.T) string {
	t.Helper()
	token := strings.Repeat("ab", 32)
	raw, err := hex.DecodeString(token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("bearer token must be 32-byte hex: %v len=%d", err, len(raw))
	}
	path := filepath.Join(t.TempDir(), "labdns-token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runServeExposure(t *testing.T, configPath string, extra ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &syncBuf{}
	errBuf := &syncBuf{}
	args := append([]string{"--config", configPath}, extra...)
	done := make(chan int, 1)
	go func() {
		done <- serve(ctx, args, out, errBuf)
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	ready := false
	for !ready {
		select {
		case <-ticker.C:
			if strings.Contains(out.String(), "labdns: listening ") || strings.Contains(errBuf.String(), "labdns serve:") {
				ready = true
			}
		case <-timer.C:
			ready = true
		}
	}
	cancel()
	select {
	case code = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not return")
	}
	return out.String(), errBuf.String(), code
}

func assertExposureWarn(t *testing.T, stdout, stderr string, code int) {
	t.Helper()
	if code != 0 {
		t.Fatalf("exit %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "labdns: shutting down") {
		t.Fatalf("missing shutdown: %q", stdout)
	}
	listen := exposureListenLine(t, stdout)
	warns := exposureWarnLines(stdout)
	if len(warns) != 1 {
		t.Fatalf("warnings=%d stdout=%q", len(warns), stdout)
	}
	if strings.Index(stdout, "labdns: listening ") > strings.Index(stdout, "labdns: warning:") {
		t.Fatalf("warning before listening line: %q", stdout)
	}
	mgmt := parseListenField(t, listen, "management")
	want := "labdns: warning: dev-loopback-unauth management bound to " + mgmt + " (not loopback); any loopback peer, including a same-host reverse proxy or SSH tunnel, is administrator; set profile: bearer"
	if warns[0] != want {
		t.Fatalf("warning=%q want=%q", warns[0], want)
	}
}

func assertExposureQuiet(t *testing.T, stdout, stderr string, code int) {
	t.Helper()
	if code != 0 {
		t.Fatalf("exit %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "labdns: shutting down") {
		t.Fatalf("missing shutdown: %q", stdout)
	}
	exposureListenLine(t, stdout)
	if warns := exposureWarnLines(stdout); len(warns) != 0 {
		t.Fatalf("warnings=%q stdout=%q", warns, stdout)
	}
}

func assertExposureFail(t *testing.T, stdout, stderr string, code int) {
	t.Helper()
	if code != 1 {
		t.Fatalf("exit %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "labdns serve:") || !strings.Contains(stderr, "management listen") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func exposureListenLine(t *testing.T, stdout string) string {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "labdns: listening ") {
			return line
		}
	}
	t.Fatalf("no listening line in %q", stdout)
	return ""
}

func exposureWarnLines(stdout string) []string {
	var lines []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "labdns: warning:") {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestServeExposureWarnBoundAddress covers binds the main table does not:
// a concrete non-loopback local address (not a wildcard), the IPv6
// wildcard, and a hostname other than "localhost" that resolves to loopback.
// The first case fails a "warn only on wildcard" predicate; the IPv6
// wildcard fails an IPv4-only "0.0.0.0" check; the hostname case fails a
// predicate that classifies the configured string instead of the bound
// address.
func TestServeExposureWarnBoundAddress(t *testing.T) {
	const dns = "127.0.0.1:0"

	t.Run("non-loopback local address", func(t *testing.T) {
		ip := nonLoopbackLocalIP(t)
		if ip == "" {
			t.Skip("no bindable non-loopback unicast address on this host")
		}
		t.Logf("non-loopback local address %s", ip)
		addr := net.JoinHostPort(ip, "0")
		stdout, stderr, code := runServeExposure(t, writeLocalConfig(t, dns, addr))
		if code != 0 && strings.Contains(stderr, "management listen") {
			t.Skipf("cannot listen on %s: %s", addr, strings.TrimSpace(stderr))
		}
		assertExposureWarn(t, stdout, stderr, code)
		mgmt := parseListenField(t, exposureListenLine(t, stdout), "management")
		host, _, err := net.SplitHostPort(mgmt)
		if err != nil || host != ip {
			t.Fatalf("bound %q, want host %s", mgmt, ip)
		}
	})

	t.Run("ipv6 wildcard [::]:0", func(t *testing.T) {
		stdout, stderr, code := runServeExposure(t, writeLocalConfig(t, dns, "[::]:0"))
		if code != 0 && strings.Contains(stderr, "management listen") {
			t.Skipf("cannot listen on [::]:0: %s", strings.TrimSpace(stderr))
		}
		assertExposureWarn(t, stdout, stderr, code)
		mgmt := parseListenField(t, exposureListenLine(t, stdout), "management")
		if strings.HasPrefix(mgmt, "0.0.0.0:") {
			t.Fatalf("bound %q: [::] reported as 0.0.0.0, case does not test the IPv6 form", mgmt)
		}
	})

	t.Run("hostname resolving to loopback", func(t *testing.T) {
		name := loopbackHostname(t)
		if name == "" {
			t.Skip("no hostname other than localhost resolves only to loopback here")
		}
		t.Logf("loopback hostname %s", name)
		addr := net.JoinHostPort(name, "0")
		stdout, stderr, code := runServeExposure(t, writeLocalConfig(t, dns, addr))
		assertExposureQuiet(t, stdout, stderr, code)
		mgmt := parseListenField(t, exposureListenLine(t, stdout), "management")
		if !auth.IsLoopback(mgmt) {
			t.Fatalf("%s bound %q, not loopback", name, mgmt)
		}
	})
}

// nonLoopbackLocalIP returns a non-loopback, non-link-local unicast address
// of this host, or "" when there is none. IPv4 is preferred.
func nonLoopbackLocalIP(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	var v6 string
	for _, a := range addrs {
		pfx, err := netip.ParsePrefix(a.String())
		if err != nil {
			continue
		}
		ip := pfx.Addr()
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		if ip.Is4() {
			return ip.String()
		}
		if v6 == "" {
			v6 = ip.String()
		}
	}
	return v6
}

// loopbackHostname returns a name, not "localhost" itself, whose every
// resolved address is loopback, or "" when none of the candidates does.
func loopbackHostname(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"labdns-test.localhost", "ip6-localhost", "ip6-loopback", "localhost.localdomain", "localhost."} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ips, err := net.DefaultResolver.LookupHost(ctx, name)
		cancel()
		if err != nil || len(ips) == 0 {
			continue
		}
		ok := true
		for _, ip := range ips {
			if !auth.IsLoopback(ip) {
				ok = false
				break
			}
		}
		if ok {
			return name
		}
	}
	return ""
}
