package rest

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// Authenticator validates a bearer token. PR-14 replaces the stub.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (auth.Actor, error)
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(ctx context.Context, token string) (auth.Actor, error)

// Authenticate calls f.
func (f AuthenticatorFunc) Authenticate(ctx context.Context, token string) (auth.Actor, error) {
	return f(ctx, token)
}

func (s *Server) authenticate(r *http.Request, cap capabilities.Capability) (auth.Actor, error) {
	// Process-local probes stay reachable for kubelet/compose healthchecks.
	if cap.RESTOnly && (cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady) {
		return auth.Actor{ID: "probe", Class: "startup"}, nil
	}
	if tok, ok := bearerToken(r.Header.Get("Authorization")); ok {
		if s.authn != nil {
			return s.authn.Authenticate(r.Context(), tok)
		}
		return auth.Actor{ID: "bearer", Class: "token"}, nil
	}
	if isLoopback(r.RemoteAddr) {
		return auth.Actor{ID: "loopback", Class: "loopback"}, nil
	}
	return auth.Actor{}, domainerr.Unauthenticated("authentication required")
}

func bearerToken(h string) (string, bool) {
	const pfx = "Bearer "
	if !strings.HasPrefix(h, pfx) && !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return "", false
	}
	// Preserve the token as given after the first space.
	i := strings.IndexByte(h, ' ')
	if i < 0 {
		return "", false
	}
	tok := strings.TrimSpace(h[i+1:])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func isLoopback(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}
