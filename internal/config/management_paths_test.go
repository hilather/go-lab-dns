package config

import "testing"

func TestManagementListenerPathsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		rest, mcp string
		valid     bool
	}{
		{"/v1", "/mcp", true}, {"/v1", "/control/mcp", true}, {"/v1", "/api/mcp", true}, {"/v1", "/api/mcp/", true}, {"/v1", "/mcp/", true}, {"/v1", "/v1/", false}, {"/v1", "/assets/", false}, {"/v1", "/login", false}, {"/v1", "/assets/x.js", false}, {"/v1", "/zones/detail", false}, {"/other", "/mcp", false},
		{"/v1", "/", false}, {"/v1", "/v1", false}, {"/v1", "/v1/state", false},
		{"/v1", "/mcp/{bad", false}, {"/v1", "GET /mcp", false}, {"/v1", "//mcp", false}, {"/v1", "/x/../mcp", false}, {"/v1", "/mcp?query", false},
	} {
		t.Run(tc.rest+tc.mcp, func(t *testing.T) {
			state := minimalState(t)
			state.Spec.Listeners.Management.RESTPath = tc.rest
			state.Spec.Listeners.Management.MCPPath = tc.mcp
			err := Validate(state)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
