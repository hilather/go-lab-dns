package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// TestServeBearerProfileLoopbackRequiresToken guards the production wiring:
// serveFromConfig passes *auth.Policy (not a bare AuthenticatorFunc) into
// REST, so a loopback peer under profile bearer must present the token file.
func TestServeBearerProfileLoopbackRequiresToken(t *testing.T) {
	path := ephemeralPackSample(t)
	tokenPath := filepath.Join(t.TempDir(), "labdns-token")
	const token = "serve-bearer-token"
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state.Spec.Management.Auth.Profile = auth.ProfileBearer
	state.Spec.Management.Auth.SecretRef = tokenPath
	body, err := config.CanonicalJSON(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := serveFromConfig(context.Background(), serveFlags{Config: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })

	base := "http://" + rt.MgmtAddr()
	noAuth := getMgmt(t, base+"/v1/state", "")
	if noAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("loopback /v1/state status=%d body=%s", noAuth.StatusCode, noAuth.Body)
	}
	if noAuth.Code != "unauthenticated" {
		t.Fatalf("code=%q body=%s", noAuth.Code, noAuth.Body)
	}
	with := getMgmt(t, base+"/v1/state", token)
	if with.StatusCode != http.StatusOK {
		t.Fatalf("bearer /v1/state status=%d body=%s", with.StatusCode, with.Body)
	}
	for _, path := range []string{"/v1/health/live", "/v1/health/ready"} {
		probe := getMgmt(t, base+path, "")
		if probe.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, probe.StatusCode, probe.Body)
		}
	}
}

// TestServeExplicitDevLoopbackUnauthUnchanged pairs the unchanged explicit
// profile with the omitted-profile contrast. On the pre-flip base, (b) is 200.
func TestServeExplicitDevLoopbackUnauthUnchanged(t *testing.T) {
	const token = "loopback-contrast-token"
	tokenPath := filepath.Join(t.TempDir(), "labdns-token")
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := auth.NewPolicy(auth.PolicyConfig{SecretRef: tokenPath})
	if err != nil {
		t.Fatal(err)
	}
	if p.Profile() != auth.ProfileBearer {
		t.Fatalf("NewPolicy profile=%s", p.Profile())
	}

	explicit := writeLocalConfigAuth(t, "127.0.0.1:0", "127.0.0.1:0", "dev-loopback-unauth", tokenPath)
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: explicit})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	if rt.mgmtProfile != auth.ProfileDevLoopbackUnauth {
		t.Fatalf("explicit profile=%s", rt.mgmtProfile)
	}
	base := "http://" + rt.MgmtAddr()
	noAuth := getMgmt(t, base+"/v1/state", "")
	if noAuth.StatusCode != http.StatusOK {
		t.Fatalf("explicit loopback /v1/state status=%d body=%s", noAuth.StatusCode, noAuth.Body)
	}
	with := getMgmt(t, base+"/v1/state", token)
	if with.StatusCode != http.StatusOK {
		t.Fatalf("explicit bearer /v1/state status=%d body=%s", with.StatusCode, with.Body)
	}

	omitted := writeLocalConfigAuth(t, "127.0.0.1:0", "127.0.0.1:0", "", tokenPath)
	rt2, err := serveFromConfig(context.Background(), serveFlags{Config: omitted})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt2.Shutdown(context.Background()) })
	if rt2.mgmtProfile != auth.ProfileBearer {
		t.Fatalf("omitted profile=%s", rt2.mgmtProfile)
	}
	base2 := "http://" + rt2.MgmtAddr()
	denied := getMgmt(t, base2+"/v1/state", "")
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("omitted loopback /v1/state status=%d body=%s", denied.StatusCode, denied.Body)
	}
	for _, path := range []string{"/v1/health/live", "/v1/health/ready"} {
		probe := getMgmt(t, base2+path, "")
		if probe.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, probe.StatusCode, probe.Body)
		}
	}
}

func TestBindManagementAuth(t *testing.T) {
	authn, pol, construct, err := bindManagementAuth(nil)
	if err != nil || construct || authn != nil || pol != nil {
		t.Fatalf("nil snap: authn=%v pol=%v construct=%v err=%v", authn, pol, construct, err)
	}
	authn, pol, construct, err = bindManagementAuth(&snapshot.Snapshot{})
	if err != nil || construct || authn != nil || pol != nil {
		t.Fatalf("nil canonical: authn=%v pol=%v construct=%v err=%v", authn, pol, construct, err)
	}

	snap := &snapshot.Snapshot{Canonical: &model.State{}}
	_, _, construct, err = bindManagementAuth(snap)
	if err == nil || construct {
		t.Fatalf("empty profile constructed: construct=%v err=%v", construct, err)
	}
	if err.Error() != "validation_failed: bearer profile has no usable token; set spec.management.auth.secretRef to a token file, or set profile: dev-loopback-unauth" {
		t.Fatalf("err=%v", err)
	}

	snap.Canonical.Spec.Management.Auth.Profile = auth.ProfileDevLoopbackUnauth
	authn, pol, construct, err = bindManagementAuth(snap)
	if err != nil || !construct || pol == nil || authn != pol {
		t.Fatalf("explicit dev-loopback-unauth: authn=%v pol=%v construct=%v err=%v", authn, pol, construct, err)
	}
	if pol.Profile() != auth.ProfileDevLoopbackUnauth {
		t.Fatalf("profile=%s", pol.Profile())
	}

	snap.Canonical.Spec.Management.Auth.Profile = auth.ProfileBearer
	if _, _, _, err = bindManagementAuth(snap); err == nil {
		t.Fatal("bearer without tokens constructed a listener authenticator")
	}
}

type mgmtResp struct {
	StatusCode int
	Code       string
	Body       string
}

func getMgmt(t *testing.T, url, bearer string) mgmtResp {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := mgmtResp{StatusCode: resp.StatusCode, Body: string(raw)}
	var problem map[string]any
	if json.Unmarshal(raw, &problem) == nil {
		out.Code, _ = problem["code"].(string)
	}
	return out
}
