package mcp

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// validateProtocolVersion pins first GA to 2026-07-28. Older SDK revisions
// still speak 2025-11-25; claiming them would violate ADR 0006.
func validateProtocolVersion(r *http.Request) error {
	ver := strings.TrimSpace(r.Header.Get(headerProtocolVersion))
	if ver == "" {
		return domainerr.UnsupportedProtocolVersion("MCP-Protocol-Version is required; only " + ProtocolVersion + " is supported")
	}
	if ver != ProtocolVersion {
		return domainerr.UnsupportedProtocolVersion("unsupported MCP protocol version " + ver + "; only " + ProtocolVersion + " is supported")
	}
	return nil
}
