package rest

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/capabilities"
)

func TestMatchParameterizedAndColonSuffix(t *testing.T) {
	routes := compileRoutes(capabilities.All())
	cases := []struct {
		method, path string
		wantParam    string
		wantVal      string
	}{
		{"GET", "/v1/zones/lab-zone", "zoneId", "lab-zone"},
		{"GET", "/v1/zones/lab-zone/records/ns1-a", "recordId", "ns1-a"},
		{"GET", "/v1/chaos/policies", "", ""},
		{"GET", "/v1/chaos/policies/slow-tools", "policyId", "slow-tools"},
		{"POST", "/v1/chaos/policies/slow-tools:activate", "id", "slow-tools"},
		{"POST", "/v1/chaos/policies/slow-tools:deactivate", "id", "slow-tools"},
		{"POST", "/v1/chaos/policies/slow-tools:expire", "id", "slow-tools"},
		{"POST", "/v1/resolve:explain", "", ""},
	}
	for _, tc := range cases {
		rt, params, pathOK, methodOK := matchRoute(routes, tc.method, tc.path)
		if !pathOK || !methodOK {
			t.Errorf("%s %s pathOK=%v methodOK=%v", tc.method, tc.path, pathOK, methodOK)
			continue
		}
		if tc.wantParam != "" && params[tc.wantParam] != tc.wantVal {
			t.Errorf("%s %s params=%v want %s=%s (route %s)", tc.method, tc.path, params, tc.wantParam, tc.wantVal, rt.path)
		}
	}
}
