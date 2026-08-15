package rest

import (
	"net/http"
	"testing"
	"time"
)

func TestChaosEmergencyRoute(t *testing.T) {
	s, _ := newTestServerFixture(t, "pack-sample.yaml")
	h := s.Handler()

	st := doLoopback(t, h, http.MethodGet, "/v1/chaos/status", "")
	requireStatus(t, st, http.StatusOK)
	if decodeJSON(t, st)["emergencyDisabled"] == true {
		t.Fatal("fixture should start with emergency off")
	}

	pols := doLoopback(t, h, http.MethodGet, "/v1/chaos/policies", "")
	requireStatus(t, pols, http.StatusOK)
	one := doLoopback(t, h, http.MethodGet, "/v1/chaos/policies/slow-tools", "")
	requireStatus(t, one, http.StatusOK)

	rev := decodeJSON(t, doLoopback(t, h, http.MethodGet, "/v1/state", ""))["runtimeRevision"].(string)
	act := doLoopback(t, h, http.MethodPost, "/v1/chaos/policies/slow-tools:activate",
		`{"expectedRevision":"`+rev+`","reason":"lab"}`)
	requireStatus(t, act, http.StatusOK)
	afterAct := decodeJSON(t, act)["candidateRevision"].(string)
	deact := doLoopback(t, h, http.MethodPost, "/v1/chaos/policies/slow-tools:deactivate",
		`{"expectedRevision":"`+afterAct+`","reason":"lab"}`)
	requireStatus(t, deact, http.StatusOK)
	// Re-activate so simulate/expire below still have an enabled policy.
	rev = decodeJSON(t, deact)["candidateRevision"].(string)
	act = doLoopback(t, h, http.MethodPost, "/v1/chaos/policies/slow-tools:activate",
		`{"expectedRevision":"`+rev+`","reason":"lab"}`)
	requireStatus(t, act, http.StatusOK)

	sim := doLoopback(t, h, http.MethodPost, "/v1/chaos:simulate",
		`{"name":"foo.tools.lab.example.net.","type":"A","clientContext":{"clientGroup":"test-devices"},"nonce":"sim"}`)
	requireStatus(t, sim, http.StatusOK)
	if decodeJSON(t, sim)["algorithm"] != "hash-v1" {
		t.Fatalf("simulate=%s", sim.Body.String())
	}

	liveRev := decodeJSON(t, doLoopback(t, h, http.MethodGet, "/v1/state", ""))["runtimeRevision"].(string)
	exp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	expire := doLoopback(t, h, http.MethodPost, "/v1/chaos/policies/slow-tools:expire",
		`{"expectedRevision":"`+liveRev+`","expiresAt":"`+exp+`"}`)
	requireStatus(t, expire, http.StatusOK)

	dis := doLoopback(t, h, http.MethodPost, "/v1/chaos:emergency-disable", `{"reason":"stop"}`)
	requireStatus(t, dis, http.StatusOK)
	after := doLoopback(t, h, http.MethodGet, "/v1/chaos/status", "")
	requireStatus(t, after, http.StatusOK)
	if decodeJSON(t, after)["emergencyDisabled"] != true {
		t.Fatalf("status after emergency=%s", after.Body.String())
	}

	// Chaos must not affect health.
	live := doLoopback(t, h, http.MethodGet, "/v1/health/live", "")
	requireStatus(t, live, http.StatusOK)
	ready := doLoopback(t, h, http.MethodGet, "/v1/health/ready", "")
	requireStatus(t, ready, http.StatusOK)

	en := doLoopback(t, h, http.MethodPost, "/v1/chaos:emergency-enable", `{"reason":"resume"}`)
	requireStatus(t, en, http.StatusOK)
}

func TestChaosEmergencyRemoteRequiresAuth(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRemote(t, s.Handler(), http.MethodPost, "/v1/chaos:emergency-disable", `{"reason":"x"}`, "192.0.2.8:1", "")
	requireProblem(t, rec, http.StatusUnauthorized, "unauthenticated")
}
