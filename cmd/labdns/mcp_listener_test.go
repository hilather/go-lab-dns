package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	mcpctl "github.com/hilather/go-lab-dns/internal/control/mcp"
)

// TestManagementServesMCPAlongsideREST proves serve mounts the Streamable
// HTTP MCP adapter on the management listener (listeners.management.mcpPath,
// default /mcp) without disturbing REST routing.
func TestManagementServesMCPAlongsideREST(t *testing.T) {
	cfg := writeLocalConfig(t, "127.0.0.1:0", "127.0.0.1:0")
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	base := "http://" + rt.mgmtLn.Addr().String()

	// REST still routes.
	st := getJSON(t, base+"/v1/state")
	if rev, _ := st["runtimeRevision"].(string); rev == "" {
		t.Fatalf("REST /v1/state missing runtimeRevision: %v", st)
	}

	// MCP discover answers on the shared listener.
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":%q,"io.modelcontextprotocol/clientInfo":{"name":"labdns-test","version":"dev"},"io.modelcontextprotocol/clientCapabilities":{}}}}`,
		mcpctl.ProtocolVersion)
	req, err := http.NewRequest(http.MethodPost, base+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", mcpctl.ProtocolVersion)
	req.Header.Set("Mcp-Method", "server/discover")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP discover status=%d body=%s", resp.StatusCode, raw)
	}
	payload := string(raw)
	if idx := strings.LastIndex(payload, "data:"); idx >= 0 {
		payload = strings.TrimSpace(payload[idx+len("data:"):])
	}
	var rpc struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal([]byte(payload), &rpc); err != nil {
		t.Fatalf("MCP discover response not JSON-RPC: %v body=%s", err, raw)
	}
	if rpc.Result == nil {
		t.Fatalf("MCP discover missing result: %s", raw)
	}
}
