package auth

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

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
