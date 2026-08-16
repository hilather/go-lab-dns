package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

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

func namedFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "config", "valid", name)
}

func copyNamedFixture(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(namedFixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, src, 0o444); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustBoot(t *testing.T, path string) *app.App {
	t.Helper()
	st, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	return app.New(app.Options{Store: store, Clock: clk, BootstrapPath: path})
}

func newTestServer(t *testing.T) (*Server, *app.App) {
	t.Helper()
	return newTestServerFixture(t, "empty-client-groups.yaml")
}

func newTestServerFixture(t *testing.T, name string) (*Server, *app.App) {
	t.Helper()
	svc := mustBoot(t, copyNamedFixture(t, name))
	s, err := New(Config{Service: svc, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	return s, svc
}

func doLoopback(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doRemote(t, h, method, path, body, "127.0.0.1:54321", "")
}

func doRemote(t *testing.T, h http.Handler, method, path, body, remote, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = remote
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	return out
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status=%d want %d body=%s", rec.Code, want, rec.Body.String())
	}
}

func requireProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	requireStatus(t, rec, status)
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	m := decodeJSON(t, rec)
	if got, _ := m["code"].(string); got != code {
		t.Fatalf("code=%v want %s body=%s", m["code"], code, rec.Body.String())
	}
	return m
}

func compactJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return string(b)
	}
	return buf.String()
}
