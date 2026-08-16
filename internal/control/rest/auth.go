package rest

import (
	"net/http"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
)

// Authenticator validates a bearer token. Shared implementation lives in
// internal/auth (Policy, static tokens, secret-ref files).
type Authenticator = auth.Authenticator

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc = auth.AuthenticatorFunc

func (s *Server) authenticate(r *http.Request, cap capabilities.Capability) (auth.Actor, error) {
	probe := cap.RESTOnly && (cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady)
	return auth.Identify(r.Context(), auth.IdentifyIn{
		RemoteAddr:    r.RemoteAddr,
		Authorization: r.Header.Get("Authorization"),
		Probe:         probe,
	}, s.authn)
}

func bearerToken(h string) (string, bool) { return auth.BearerToken(h) }

func isLoopback(remoteAddr string) bool { return auth.IsLoopback(remoteAddr) }
