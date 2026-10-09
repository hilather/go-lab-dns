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
