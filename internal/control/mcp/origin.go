package mcp

import (
	"net/http"

	"github.com/hilather/go-lab-dns/internal/auth"
)

// validateOrigin implements the Streamable HTTP Origin check. Shared
// default-deny policy lives in internal/auth.
func validateOrigin(r *http.Request, allowed []string) error {
	return auth.CheckOrigin(r.Header.Get(headerOrigin), allowed)
}

func originAllowed(origin string, extra []string) bool {
	return auth.OriginAllowed(origin, extra)
}
