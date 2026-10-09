package auth

import (
	"context"
	"net"
	"net/netip"
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Credential classes recorded on Actor and audit events.
const (
	ClassToken       = "token"
	ClassMTLS        = "mtls"
	ClassProxy       = "proxy"
	ClassLoopback    = "loopback"
	ClassLocalSignal = "local-signal"
	ClassStartup     = "startup"
)

// Actor is the authenticated caller shared by REST, MCP, and local signals.
type Actor struct {
	ID     string
	Class  string // token | mtls | proxy | loopback | local-signal | startup | ui-session
	Role   string
	Scopes []string
	Groups []string
}

// Authenticator validates a presented bearer token.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (Actor, error)
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(ctx context.Context, token string) (Actor, error)

// Authenticate calls f.
func (f AuthenticatorFunc) Authenticate(ctx context.Context, token string) (Actor, error) {
	return f(ctx, token)
}

// ProfileReporter reports the serve-time auth profile. *Policy implements it.
// Identify uses the report to decide whether a loopback peer without a bearer
// is administrator. Only an explicit dev-loopback-unauth report keeps that
// exception. A wrapper Authenticator in front of *Policy must forward Profile.
// A wrapper that does not implement ProfileReporter, or that reports an empty
// or unknown profile, loses that exception. A presented bearer is still
// passed to Authenticate.
type ProfileReporter interface {
	Profile() string
}

// IdentifyIn is the transport-agnostic management identity request.
type IdentifyIn struct {
	RemoteAddr    string
	Authorization string
	// Probe skips auth so process-local health checks work off-loopback.
	Probe bool
}

// Identify applies Q-AUTH (ADR 0010). Under an explicit dev-loopback-unauth
// profile, loopback (127.0.0.1 / ::1, including IPv4-mapped) may omit a bearer
// and is administrator. Under bearer, every peer including loopback must
// present Authorization: Bearer. A nil Authenticator fails closed: every
// non-probe request is Unauthenticated ("authentication required"), whether or
// not a bearer is presented. An Authenticator that does not implement
// ProfileReporter, or that reports an empty or unknown profile, is not the
// dev exception; a presented bearer is still passed to Authenticate. Probe
// requests skip auth so health live/ready stay unauthenticated.
// X-Forwarded-For is not consulted (callers pass RemoteAddr only). MCP stdio
// uses LocalOrStdio and does not call Identify.
func Identify(ctx context.Context, in IdentifyIn, tokens Authenticator) (Actor, error) {
	if in.Probe {
		return Actor{ID: "probe", Class: ClassStartup, Role: RoleAdministrator}, nil
	}
	if tokens == nil {
		return Actor{}, domainerr.Unauthenticated("authentication required")
	}
	if tok, ok := BearerToken(in.Authorization); ok {
		a, err := tokens.Authenticate(ctx, tok)
		if err != nil {
			return Actor{}, err
		}
		return completeActor(a), nil
	}
	if IsLoopback(in.RemoteAddr) && effectiveProfile(tokens) == ProfileDevLoopbackUnauth {
		return Actor{ID: "loopback", Class: ClassLoopback, Role: RoleAdministrator}, nil
	}
	return Actor{}, domainerr.Unauthenticated("authentication required")
}

// effectiveProfile is the profile tokens reports.
// A nil Authenticator, a value that does not implement ProfileReporter, a nil
// *Policy, and an empty or unknown report are not the dev exception. Only an
// explicit ProfileDevLoopbackUnauth report keeps the loopback exception.
// (*Policy).Profile on a nil receiver returns "" (no profile), not dev-loopback-unauth.
func effectiveProfile(tokens Authenticator) string {
	reporter, ok := tokens.(ProfileReporter)
	if !ok {
		return ""
	}
	return reporter.Profile()
}

// BearerToken extracts the token from an Authorization header.
func BearerToken(h string) (string, bool) {
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

// IsLoopback reports whether remoteAddr is 127.0.0.1 or ::1 (with or without a port).
func IsLoopback(remoteAddr string) bool {
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

// RateKey is the per-source limiter key. Unparseable peers share "unknown".
func RateKey(remoteAddr string) string {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	return addr.String()
}

// completeActor fills an identity-only hook result (id/class, no role/scopes)
// so existing AuthenticatorFunc tests keep working.
func completeActor(a Actor) Actor {
	if a.Role == "" && len(a.Scopes) == 0 {
		a.Role = RoleAdministrator
	}
	return a
}

// LocalOrStdio is the implicit actor when a tool runs without Identify
// (MCP stdio developer adapter, no network peer). ADR 0010 does not change it.
func LocalOrStdio(a Actor) Actor {
	if a.ID == "" && a.Class == "" && a.Role == "" && len(a.Scopes) == 0 {
		return Actor{ID: "stdio", Class: ClassLoopback, Role: RoleAdministrator}
	}
	return a
}

// LocalSignal is the SIGUSR1 / startup-disable actor.
func LocalSignal(id string) Actor {
	if id == "" {
		id = "signal"
	}
	return Actor{ID: id, Class: ClassLocalSignal, Role: RoleAdministrator}
}

// Profile names match model.AuthSpec.
const (
	ProfileDevLoopbackUnauth = model.AuthProfileDevLoopbackUnauth
	ProfileBearer            = model.AuthProfileBearer
)
