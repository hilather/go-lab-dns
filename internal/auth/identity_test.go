package auth

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestIdentifyProfileControlsLoopback(t *testing.T) {
	bearer, err := NewPolicy(PolicyConfig{
		Profile: ProfileBearer,
		Tokens:  []Token{{Token: "good", ID: "tok", Role: RoleAdministrator}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dev, err := NewPolicy(PolicyConfig{Profile: ProfileDevLoopbackUnauth})
	if err != nil {
		t.Fatal(err)
	}
	hook := AuthenticatorFunc(func(ctx context.Context, token string) (Actor, error) {
		_ = ctx
		_ = token
		return Actor{ID: "hook", Class: ClassToken}, nil
	})

	// These addresses are loopback peers. A future IsLoopback change that
	// stops treating them as loopback would hide the bearer-profile denial.
	for _, addr := range []string{"127.0.0.1:9", "[::1]:9", "::ffff:127.0.0.1", "[::ffff:127.0.0.1]:9"} {
		if !IsLoopback(addr) {
			t.Fatalf("%s is not loopback", addr)
		}
	}

	type want struct {
		class string
		id    string
	}
	cases := []struct {
		name   string
		tokens Authenticator
		in     IdentifyIn
		ok     *want
	}{
		{"bearer loopback v4", bearer, IdentifyIn{RemoteAddr: "127.0.0.1:9"}, nil},
		{"bearer loopback v6", bearer, IdentifyIn{RemoteAddr: "[::1]:9"}, nil},
		{"bearer mapped v4", bearer, IdentifyIn{RemoteAddr: "::ffff:127.0.0.1"}, nil},
		{"bearer mapped v4 port", bearer, IdentifyIn{RemoteAddr: "[::ffff:127.0.0.1]:9"}, nil},
		{"bearer loopback token", bearer, IdentifyIn{RemoteAddr: "127.0.0.1:9", Authorization: "Bearer good"}, &want{ClassToken, "tok"}},
		{"bearer remote", bearer, IdentifyIn{RemoteAddr: "192.0.2.10:9"}, nil},
		{"dev loopback", dev, IdentifyIn{RemoteAddr: "127.0.0.1:9"}, &want{ClassLoopback, "loopback"}},
		{"dev loopback v6", dev, IdentifyIn{RemoteAddr: "[::1]:9"}, &want{ClassLoopback, "loopback"}},
		{"bearer probe", bearer, IdentifyIn{RemoteAddr: "192.0.2.10:9", Probe: true}, &want{ClassStartup, "probe"}},
		{"nil loopback", nil, IdentifyIn{RemoteAddr: "127.0.0.1:9"}, nil},
		{"nil bearer", nil, IdentifyIn{RemoteAddr: "192.0.2.10:9", Authorization: "Bearer dev-token"}, nil},
		{"func loopback", hook, IdentifyIn{RemoteAddr: "127.0.0.1:9"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Identify(context.Background(), tc.in, tc.tokens)
			if tc.ok == nil {
				de, ok := domainerr.As(err)
				if !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" {
					t.Fatalf("actor=%+v err=%v", a, err)
				}
				return
			}
			if err != nil || a.Class != tc.ok.class || a.ID != tc.ok.id {
				t.Fatalf("actor=%+v err=%v", a, err)
			}
			if tc.ok.class == ClassLoopback && !a.HasScope(ScopeDNSAdmin) {
				t.Fatalf("loopback missing admin scope: %+v", a)
			}
			if tc.ok.class == ClassToken && !a.HasScope(ScopeDNSAdmin) {
				t.Fatalf("token missing admin scope: %+v", a)
			}
		})
	}
}

func TestIdentifyLoopback(t *testing.T) {
	dev, err := NewPolicy(PolicyConfig{Profile: ProfileDevLoopbackUnauth})
	if err != nil {
		t.Fatal(err)
	}
	a, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "127.0.0.1:9"}, dev)
	if err != nil {
		t.Fatal(err)
	}
	if a.Class != ClassLoopback || !a.HasScope(ScopeDNSAdmin) {
		t.Fatalf("%+v", a)
	}
	a6, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "[::1]:9"}, dev)
	if err != nil || !a6.HasScope(ScopeDNSRead) {
		t.Fatalf("%+v %v", a6, err)
	}
}

func TestIdentifyRemoteRequiresBearer(t *testing.T) {
	_, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "192.0.2.10:9"}, nil)
	if err == nil {
		t.Fatal("expected unauthenticated")
	}
	if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeUnauthenticated {
		t.Fatalf("err=%v", err)
	}
}

func TestIdentifyRemoteBearerUnconfigured(t *testing.T) {
	_, err := Identify(context.Background(), IdentifyIn{
		RemoteAddr:    "192.0.2.10:9",
		Authorization: "Bearer dev-token",
	}, nil)
	if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" {
		t.Fatalf("err=%v", err)
	}
}

// profileAuth is a test Authenticator that reports a fixed profile.
type profileAuth struct {
	profile string
}

func (p profileAuth) Authenticate(ctx context.Context, token string) (Actor, error) {
	_ = ctx
	if token == "" {
		return Actor{}, domainerr.Unauthenticated("authentication required")
	}
	return Actor{ID: "hook", Class: ClassToken}, nil
}

func (p profileAuth) Profile() string { return p.profile }

func TestIdentifyNilAuthenticatorFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		in   IdentifyIn
	}{
		{"loopback no bearer", IdentifyIn{RemoteAddr: "127.0.0.1:9"}},
		{"loopback v6 no bearer", IdentifyIn{RemoteAddr: "[::1]:9"}},
		{"loopback bearer", IdentifyIn{RemoteAddr: "127.0.0.1:9", Authorization: "Bearer dev-token"}},
		{"remote bearer", IdentifyIn{RemoteAddr: "192.0.2.10:9", Authorization: "Bearer dev-token"}},
		{"remote no bearer", IdentifyIn{RemoteAddr: "192.0.2.10:9"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Identify(context.Background(), tc.in, nil)
			if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" || a.ID != "" {
				t.Fatalf("actor=%+v err=%v", a, err)
			}
		})
	}
	var pol *Policy
	if pol.Profile() != "" {
		t.Fatalf("nil policy profile %q", pol.Profile())
	}
	a, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "127.0.0.1:9"}, pol)
	if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" {
		t.Fatalf("nil *Policy actor=%+v err=%v", a, err)
	}
	probe, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "192.0.2.10:9", Probe: true}, nil)
	if err != nil || probe.ID != "probe" || probe.Class != ClassStartup {
		t.Fatalf("probe actor=%+v err=%v", probe, err)
	}
}

func TestIdentifyTypedNilPolicyBearerUnauthenticated(t *testing.T) {
	var pol *Policy
	a, err := Identify(context.Background(), IdentifyIn{
		RemoteAddr:    "127.0.0.1:9",
		Authorization: "Bearer dev-token",
	}, pol)
	if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" || a.ID != "" {
		t.Fatalf("actor=%+v err=%v", a, err)
	}
}

func TestIdentifyMissingProfileFailsClosed(t *testing.T) {
	hook := AuthenticatorFunc(func(ctx context.Context, token string) (Actor, error) {
		_ = ctx
		return Actor{ID: "hook", Class: ClassToken}, nil
	})
	denied := []struct {
		name   string
		tokens Authenticator
		in     IdentifyIn
	}{
		{"func loopback", hook, IdentifyIn{RemoteAddr: "127.0.0.1:9"}},
		{"empty profile loopback", profileAuth{}, IdentifyIn{RemoteAddr: "127.0.0.1:9"}},
		{"unknown profile loopback", profileAuth{profile: "nope"}, IdentifyIn{RemoteAddr: "[::1]:9"}},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Identify(context.Background(), tc.in, tc.tokens)
			if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" || a.ID != "" {
				t.Fatalf("actor=%+v err=%v", a, err)
			}
		})
	}

	t.Run("explicit dev loopback", func(t *testing.T) {
		a, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "127.0.0.1:9"}, profileAuth{profile: ProfileDevLoopbackUnauth})
		if err != nil || a.Class != ClassLoopback || a.ID != "loopback" || !a.HasScope(ScopeDNSAdmin) {
			t.Fatalf("actor=%+v err=%v", a, err)
		}
	})
	t.Run("func bearer still authenticates", func(t *testing.T) {
		a, err := Identify(context.Background(), IdentifyIn{
			RemoteAddr:    "192.0.2.10:9",
			Authorization: "Bearer dev-token",
		}, hook)
		if err != nil || a.ID != "hook" || a.Class != ClassToken || !a.HasScope(ScopeDNSAdmin) {
			t.Fatalf("actor=%+v err=%v", a, err)
		}
	})
}

func TestIdentifyProbeSkipsAuth(t *testing.T) {
	a, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "192.0.2.10:9", Probe: true}, nil)
	if err != nil || a.Class != ClassStartup {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestIdentifyPolicyRejectsUnknownToken(t *testing.T) {
	p, err := NewPolicy(PolicyConfig{Tokens: []Token{{Token: "good", Role: RoleViewer}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Identify(context.Background(), IdentifyIn{
		RemoteAddr:    "192.0.2.10:9",
		Authorization: "Bearer nope",
	}, p)
	if err == nil {
		t.Fatal("expected reject")
	}
	a, err := Identify(context.Background(), IdentifyIn{
		RemoteAddr:    "192.0.2.10:9",
		Authorization: "Bearer good",
	}, p)
	if err != nil || a.Role != RoleViewer || !a.HasScope(ScopeDNSRead) || a.HasScope(ScopeDNSWrite) {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestIsLoopbackAndRateKey(t *testing.T) {
	if !IsLoopback("127.0.0.1:1") || !IsLoopback("[::1]:80") {
		t.Fatal("loopback")
	}
	if IsLoopback("192.0.2.1:1") {
		t.Fatal("remote")
	}
	if RateKey("192.0.2.8:9") != "192.0.2.8" {
		t.Fatal(RateKey("192.0.2.8:9"))
	}
	if RateKey("not-an-addr") != "unknown" {
		t.Fatal(RateKey("not-an-addr"))
	}
}

func TestBearerToken(t *testing.T) {
	tok, ok := BearerToken("Bearer abc")
	if !ok || tok != "abc" {
		t.Fatalf("%q %v", tok, ok)
	}
	if _, ok := BearerToken("Basic x"); ok {
		t.Fatal("basic")
	}
	if _, ok := BearerToken("Bearer "); ok {
		t.Fatal("empty")
	}
}
