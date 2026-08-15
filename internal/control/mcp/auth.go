package mcp

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func (s *Server) authenticate(r *http.Request) (auth.Actor, error) {
	if tok, ok := bearerToken(r.Header.Get(headerAuthorization)); ok {
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
	if !strings.HasPrefix(h, "Bearer ") && !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return "", false
	}
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
