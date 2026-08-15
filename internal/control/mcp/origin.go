package mcp

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// validateOrigin implements the Streamable HTTP Origin check. A missing Origin
// is allowed (official SDK / curl). A present Origin must be loopback or on
// the configured allowlist; anything else is DNS-rebinding.
func validateOrigin(r *http.Request, allowed []string) error {
	origin := strings.TrimSpace(r.Header.Get(headerOrigin))
	if origin == "" {
		return nil
	}
	if originAllowed(origin, allowed) {
		return nil
	}
	return domainerr.Forbidden("origin not allowed")
}

func originAllowed(origin string, extra []string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if isLoopbackHost(host) {
		return true
	}
	for _, a := range extra {
		if origin == a {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}
