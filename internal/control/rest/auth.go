package rest

import (
	"net/http"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// Authenticator validates a bearer token. Shared implementation lives in
// internal/auth (Policy, static tokens, secret-ref files).
type Authenticator = auth.Authenticator

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc = auth.AuthenticatorFunc

func (s *Server) authenticate(r *http.Request, cap capabilities.Capability) (auth.Actor, error) {
	probe := cap.RESTOnly && (cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady)
	if probe {
		return auth.Identify(r.Context(), auth.IdentifyIn{
			RemoteAddr:    r.RemoteAddr,
			Authorization: r.Header.Get("Authorization"),
			Probe:         true,
		}, s.authn)
	}
	if _, ok := auth.BearerToken(r.Header.Get("Authorization")); ok {
		return auth.Identify(r.Context(), auth.IdentifyIn{
			RemoteAddr:    r.RemoteAddr,
			Authorization: r.Header.Get("Authorization"),
		}, s.authn)
	}
	if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" {
		if sess, ok := s.sessions.Lookup(c.Value); ok {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if !auth.CSRFEqual(r.Header.Get(auth.CSRFHeader), sess.CSRF) {
					return auth.Actor{}, domainerr.Forbidden("invalid CSRF token")
				}
			}
			return sess.Actor, nil
		}
		// Present but not live: do not Identify on session POST (loopback would
		// mint administrator). First login omits the cookie.
		if cap.ID == capabilities.Session && r.Method == http.MethodPost {
			return auth.Actor{}, domainerr.Unauthenticated("authentication required")
		}
	}
	return auth.Identify(r.Context(), auth.IdentifyIn{
		RemoteAddr:    r.RemoteAddr,
		Authorization: r.Header.Get("Authorization"),
	}, s.authn)
}

func isLoopback(remoteAddr string) bool { return auth.IsLoopback(remoteAddr) }
