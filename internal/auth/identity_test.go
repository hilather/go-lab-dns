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
		{"nil loopback", nil, IdentifyIn{RemoteAddr: "127.0.0.1:9"}, &want{ClassLoopback, "loopback"}},
		{"func loopback", hook, IdentifyIn{RemoteAddr: "127.0.0.1:9"}, &want{ClassLoopback, "loopback"}},
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
	a, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "127.0.0.1:9"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Class != ClassLoopback || !a.HasScope(ScopeDNSAdmin) {
		t.Fatalf("%+v", a)
	}
	a6, err := Identify(context.Background(), IdentifyIn{RemoteAddr: "[::1]:9"}, nil)
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
	a, err := Identify(context.Background(), IdentifyIn{
		RemoteAddr:    "192.0.2.10:9",
		Authorization: "Bearer dev-token",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Class != ClassToken || !a.HasScope(ScopeDNSAdmin) {
		t.Fatalf("%+v", a)
	}
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
