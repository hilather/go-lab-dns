package auth

import (
	"net/http"
	"net/netip"
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// CheckOrigin implements the management Origin policy. A missing Origin is
// allowed (SDK/curl). A present Origin must be loopback or on allowed.
// This is DNS-rebinding default-deny; it is not a CORS allowlist.
func CheckOrigin(origin string, allowed []string) error {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return nil
	}
	if OriginAllowed(origin, allowed) {
		return nil
	}
	return domainerr.Forbidden("origin not allowed")
}

// OriginAllowed reports whether origin is loopback http(s) or exactly listed.
func OriginAllowed(origin string, extra []string) bool {
	u, err := parseHTTPOrigin(origin)
	if err != nil {
		return false
	}
	if isLoopbackHost(u.host) {
		return true
	}
	for _, a := range extra {
		if origin == a {
			return true
		}
	}
	return false
}

type parsedOrigin struct {
	scheme string
	host   string
}

func parseHTTPOrigin(origin string) (parsedOrigin, error) {
	// net/url is fine here; we only accept http(s) with a host.
	// Inline parse keeps this package free of request objects.
	scheme, rest, ok := strings.Cut(origin, "://")
	if !ok {
		return parsedOrigin{}, domainerr.Forbidden("origin not allowed")
	}
	scheme = strings.ToLower(scheme)
	if scheme != "http" && scheme != "https" {
		return parsedOrigin{}, domainerr.Forbidden("origin not allowed")
	}
	host := rest
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		host = rest[:i]
	}
	if host == "" {
		return parsedOrigin{}, domainerr.Forbidden("origin not allowed")
	}
	// Strip port for hostname checks; IPv6 is [addr]:port.
	hostname := host
	if strings.HasPrefix(host, "[") {
		if end := strings.IndexByte(host, ']'); end > 0 {
			hostname = host[1:end]
		}
	} else if h, _, err := splitHostPortSafe(host); err == nil {
		hostname = h
	}
	return parsedOrigin{scheme: scheme, host: hostname}, nil
}

func splitHostPortSafe(hostport string) (string, string, error) {
	// Avoid net.SplitHostPort on bare hostnames without a port.
	if !strings.Contains(hostport, ":") {
		return hostport, "", domainerr.Forbidden("origin not allowed")
	}
	i := strings.LastIndexByte(hostport, ':')
	return hostport[:i], hostport[i+1:], nil
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

// CORSDenyHeaders documents that adapters must not emit allow-* headers.
// ApplyCORS writes nothing: deny-all is the absence of CORS.
func ApplyCORS(h http.Header) {
	_ = h
}
