package mcp

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
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
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

func doMCP(t *testing.T, h http.Handler, body string, hdr http.Header, remote string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, DefaultPath, strings.NewReader(body))
	req.RemoteAddr = remote
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(headerProtocolVersion, ProtocolVersion)
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doLoopbackMCP(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doMCP(t, h, body, nil, "127.0.0.1:54321")
}

func rpcCall(id int, method string, params any) string {
	p := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		p["params"] = params
	} else {
		p["params"] = map[string]any{
			"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		}
	}
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func rpcParams(args map[string]any) map[string]any {
	if args == nil {
		args = map[string]any{}
	}
	return map[string]any{
		"_meta":     map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion},
		"arguments": args,
	}
}

func decodeRPC(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	raw := rec.Body.Bytes()
	if bytes.Contains(raw, []byte("event:")) {
		// SSE: take the last data: line.
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "data:") {
				raw = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	return out
}

func requireRPCError(t *testing.T, rec *httptest.ResponseRecorder, status int, domainCode string) map[string]any {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d want %d body=%s", rec.Code, status, rec.Body.String())
	}
	m := decodeRPC(t, rec)
	errObj, _ := m["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("missing error: %s", rec.Body.String())
	}
	data, _ := errObj["data"].(map[string]any)
	if data == nil {
		t.Fatalf("missing error.data: %s", rec.Body.String())
	}
	if got, _ := data["code"].(string); got != domainCode {
		t.Fatalf("data.code=%v want %s body=%s", data["code"], domainCode, rec.Body.String())
	}
	return errObj
}

func startHTTP(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func connectClient(t *testing.T, ts *httptest.Server) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "labdns-test", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             ts.URL + DefaultPath,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(t *testing.T, cs *sdk.ClientSession, name string, args any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func structuredMap(t *testing.T, res *sdk.CallToolResult) map[string]any {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %+v", res)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("structured: %v raw=%s", err, raw)
	}
	return out
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

func mustRead(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
