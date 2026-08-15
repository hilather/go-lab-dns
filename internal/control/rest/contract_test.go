package rest

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestRoutesRegisteredFromRegistry(t *testing.T) {
	s, _ := newTestServer(t)
	if len(s.routes) == 0 {
		t.Fatal("no routes")
	}
	seen := map[string]bool{}
	for _, rt := range s.routes {
		seen[rt.method+" "+rt.path] = true
	}
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			ref := strings.ToUpper(b.Method) + " " + b.Path
			if !seen[ref] {
				t.Errorf("missing registry route %s", ref)
			}
		}
	}
}

func TestContractReads(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()

	live := doLoopback(t, h, http.MethodGet, "/v1/health/live", "")
	requireStatus(t, live, http.StatusOK)
	if decodeJSON(t, live)["status"] != "ok" {
		t.Fatalf("live=%s", live.Body.String())
	}

	ready := doLoopback(t, h, http.MethodGet, "/v1/health/ready", "")
	requireStatus(t, ready, http.StatusOK)

	ver := doLoopback(t, h, http.MethodGet, "/v1/version", "")
	requireStatus(t, ver, http.StatusOK)
	if decodeJSON(t, ver)["protocols"] == nil {
		t.Fatalf("version=%s", ver.Body.String())
	}

	caps := doLoopback(t, h, http.MethodGet, "/v1/capabilities", "")
	requireStatus(t, caps, http.StatusOK)
	clist, _ := decodeJSON(t, caps)["capabilities"].([]any)
	if len(clist) == 0 {
		t.Fatal("empty capabilities")
	}

	st := doLoopback(t, h, http.MethodGet, "/v1/status", "")
	requireStatus(t, st, http.StatusOK)
	if decodeJSON(t, st)["revisions"] == nil {
		t.Fatalf("status=%s", st.Body.String())
	}

	schema := doLoopback(t, h, http.MethodGet, "/v1/schema/config", "")
	requireStatus(t, schema, http.StatusOK)
	if !strings.Contains(schema.Body.String(), "labdns.dev/v1alpha1") {
		t.Fatalf("schema missing api version")
	}

	docs := doLoopback(t, h, http.MethodGet, "/v1/docs/dns-semantics", "")
	requireStatus(t, docs, http.StatusOK)
	if !strings.Contains(docs.Header().Get("Content-Type"), "markdown") {
		t.Fatalf("docs content-type=%s", docs.Header().Get("Content-Type"))
	}
	safety := doLoopback(t, h, http.MethodGet, "/v1/docs/chaos-safety", "")
	requireStatus(t, safety, http.StatusOK)

	state := doLoopback(t, h, http.MethodGet, "/v1/state", "")
	requireStatus(t, state, http.StatusOK)
	sm := decodeJSON(t, state)
	if sm["runtimeRevision"] == "" {
		t.Fatalf("state=%s", state.Body.String())
	}

	zones := doLoopback(t, h, http.MethodGet, "/v1/zones", "")
	requireStatus(t, zones, http.StatusOK)
	zone := doLoopback(t, h, http.MethodGet, "/v1/zones/lab-zone", "")
	requireStatus(t, zone, http.StatusOK)
	missing := doLoopback(t, h, http.MethodGet, "/v1/zones/nope", "")
	requireProblem(t, missing, http.StatusNotFound, "not_found")

	recs := doLoopback(t, h, http.MethodGet, "/v1/zones/lab-zone/records?limit=1", "")
	requireStatus(t, recs, http.StatusOK)
	rm := decodeJSON(t, recs)
	if rm["nextCursor"] == nil && len(mustSlice(rm["records"])) < 1 {
		t.Fatalf("records=%s", recs.Body.String())
	}
	rec := doLoopback(t, h, http.MethodGet, "/v1/zones/lab-zone/records/ns1-a", "")
	requireStatus(t, rec, http.StatusOK)
	if decodeJSON(t, rec)["id"] != "ns1-a" {
		t.Fatalf("record=%s", rec.Body.String())
	}

	resolve := doLoopback(t, h, http.MethodPost, "/v1/resolve", `{"name":"ns1.lab.example.net","type":"A"}`)
	requireStatus(t, resolve, http.StatusOK)
	if decodeJSON(t, resolve)["result"] == nil {
		t.Fatalf("resolve=%s", resolve.Body.String())
	}

	explain := doLoopback(t, h, http.MethodPost, "/v1/resolve:explain", `{"name":"ns1.lab.example.net.","type":"A","clientContext":{"client":"127.0.0.1","transport":"udp"}}`)
	requireStatus(t, explain, http.StatusOK)

	_ = svc
}

func TestContractMutations(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()
	st := doLoopback(t, h, http.MethodGet, "/v1/state", "")
	rev := decodeJSON(t, st)["runtimeRevision"].(string)

	op := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab-zone"},
		Value:  json.RawMessage(`{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}`),
	}
	body := compactJSON(t, map[string]any{
		"expectedRevision": rev,
		"reason":           "add www",
		"operations":       []model.Operation{op},
	})

	val := doLoopback(t, h, http.MethodPost, "/v1/state:validate", `{"operations":[{"op":"add","target":{"kind":"record","id":"www-a","zoneId":"lab-zone"},"value":{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}}]}`)
	requireStatus(t, val, http.StatusOK)

	gotState := doLoopback(t, h, http.MethodGet, "/v1/state", "")
	requireStatus(t, gotState, http.StatusOK)
	canonical, _ := decodeJSON(t, gotState)["canonical"].(map[string]any)
	if canonical == nil {
		t.Fatalf("state missing canonical: %s", gotState.Body.String())
	}
	roundTrip, err := json.Marshal(map[string]any{"state": canonical})
	if err != nil {
		t.Fatal(err)
	}
	rt := doLoopback(t, h, http.MethodPost, "/v1/state:validate", string(roundTrip))
	requireStatus(t, rt, http.StatusOK)

	plan := doLoopback(t, h, http.MethodPost, "/v1/changes:plan", body)
	requireStatus(t, plan, http.StatusOK)
	if decodeJSON(t, plan)["candidateRevision"] == rev {
		t.Fatal("plan did not change revision")
	}

	bad := doLoopback(t, h, http.MethodPost, "/v1/changes:apply", compactJSON(t, map[string]any{
		"expectedRevision": "sha256:deadbeef",
		"operations":       []model.Operation{op},
	}))
	requireProblem(t, bad, http.StatusConflict, "revision_conflict")

	apply := doLoopback(t, h, http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, apply, http.StatusOK)
	am := decodeJSON(t, apply)
	if am["applied"] != true {
		t.Fatalf("apply=%s", apply.Body.String())
	}

	exp := doLoopback(t, h, http.MethodGet, "/v1/state:export?format=yaml", "")
	requireStatus(t, exp, http.StatusOK)
	if !strings.Contains(exp.Header().Get("Content-Type"), "yaml") {
		t.Fatalf("export type=%s", exp.Header().Get("Content-Type"))
	}
	if !strings.Contains(exp.Body.String(), "www-a") {
		t.Fatalf("export missing www-a: %s", exp.Body.String())
	}
	expj := doLoopback(t, h, http.MethodGet, "/v1/state:export?format=json", "")
	requireStatus(t, expj, http.StatusOK)

	reset := doLoopback(t, h, http.MethodPost, "/v1/state:reset", `{"reason":"test"}`)
	requireStatus(t, reset, http.StatusOK)
	live := doLoopback(t, h, http.MethodGet, "/v1/zones/lab-zone/records/www-a", "")
	requireProblem(t, live, http.StatusNotFound, "not_found")
	_ = svc
}

func TestIdempotentApplySameKey(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	rev := decodeJSON(t, doLoopback(t, h, http.MethodGet, "/v1/state", ""))["runtimeRevision"].(string)
	body := `{"expectedRevision":"` + rev + `","idempotencyKey":"k1","reason":"add","operations":[{"op":"add","target":{"kind":"record","id":"www-a","zoneId":"lab-zone"},"value":{"id":"www-a","owner":"www","type":"A","values":["10.42.0.80"]}}]}`
	a1 := doLoopback(t, h, http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, a1, http.StatusOK)
	a2 := doLoopback(t, h, http.MethodPost, "/v1/changes:apply", body)
	requireStatus(t, a2, http.StatusOK)
	if decodeJSON(t, a1)["candidateRevision"] != decodeJSON(t, a2)["candidateRevision"] {
		t.Fatal("idempotent apply returned different revisions")
	}
	conflict := doLoopback(t, h, http.MethodPost, "/v1/changes:apply", `{"expectedRevision":"`+rev+`","idempotencyKey":"k1","reason":"other","operations":[]}`)
	requireProblem(t, conflict, http.StatusConflict, "idempotency_conflict")
}

func TestContractForwardingCacheAudit(t *testing.T) {
	s, _ := newTestServerFixture(t, "pack-sample.yaml")
	h := s.Handler()

	fp := doLoopback(t, h, http.MethodGet, "/v1/forwarding/policies", "")
	requireStatus(t, fp, http.StatusOK)
	pools := doLoopback(t, h, http.MethodGet, "/v1/upstream-pools", "")
	requireStatus(t, pools, http.StatusOK)
	ups := doLoopback(t, h, http.MethodGet, "/v1/upstreams/status", "")
	requireStatus(t, ups, http.StatusOK)
	cs := doLoopback(t, h, http.MethodGet, "/v1/cache/status", "")
	requireStatus(t, cs, http.StatusOK)
	flush := doLoopback(t, h, http.MethodPost, "/v1/cache:flush", `{"all":true}`)
	requireStatus(t, flush, http.StatusNoContent)

	audit := doLoopback(t, h, http.MethodGet, "/v1/audit", "")
	requireStatus(t, audit, http.StatusOK)
	missing := doLoopback(t, h, http.MethodGet, "/v1/audit/nope", "")
	requireProblem(t, missing, http.StatusNotFound, "not_found")
}

func TestUnknownPathAndMethod(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	requireProblem(t, doLoopback(t, h, http.MethodGet, "/v1/nope", ""), http.StatusNotFound, "not_found")
	rec := doLoopback(t, h, http.MethodDelete, "/v1/state", "")
	requireProblem(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if rec.Header().Get("Allow") == "" {
		t.Fatal("missing Allow")
	}
}

func TestNoPermissiveCORS(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doLoopback(t, s.Handler(), http.MethodGet, "/v1/health/live", "")
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("permissive CORS: %v", rec.Header())
	}
}

func TestRequestIDEcho(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health/live", nil)
	req.Header.Set("X-Request-ID", "req-test-1")
	req.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("X-Request-ID") != "req-test-1" {
		t.Fatalf("request id=%s", rec.Header().Get("X-Request-ID"))
	}
}

func TestDefaultListenAddr(t *testing.T) {
	s, _ := newTestServer(t)
	if s.Addr() != DefaultAddr || DefaultAddr != ":8080" {
		t.Fatalf("addr=%s", s.Addr())
	}
}

func TestServeLoopbackHealth(t *testing.T) {
	s, _ := newTestServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ln) }()
	t.Cleanup(func() {
		_ = s.Shutdown(context.Background())
		<-errCh
	})
	resp, err := http.Get("http://" + ln.Addr().String() + "/v1/health/live")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestPaginationInvalidCursor(t *testing.T) {
	s, _ := newTestServer(t)
	requireProblem(t, doLoopback(t, s.Handler(), http.MethodGet, "/v1/zones?cursor=abc", ""), http.StatusBadRequest, "validation_failed")
}

func mustSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
