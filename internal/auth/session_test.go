package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestSessionCreateLookupExpire(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	cur := now
	tab := NewSessionTable(SessionTableConfig{
		TTL: time.Hour,
		Max: 4,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return cur
		},
	})
	sess, err := tab.Create(Actor{ID: "loopback", Class: ClassLoopback, Role: RoleAdministrator})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == "" || sess.CSRF == "" || len(sess.ID) != 64 || len(sess.CSRF) != 64 {
		t.Fatalf("id/csrf hex: id=%q csrf=%q", sess.ID, sess.CSRF)
	}
	if sess.Actor.Class != ClassUISession || sess.Actor.ID != "loopback" || sess.Actor.Role != RoleAdministrator {
		t.Fatalf("actor=%+v", sess.Actor)
	}
	got, ok := tab.Lookup(sess.ID)
	if !ok || got.CSRF != sess.CSRF {
		t.Fatalf("lookup=%+v ok=%v", got, ok)
	}
	mu.Lock()
	cur = cur.Add(2 * time.Hour)
	mu.Unlock()
	if _, ok := tab.Lookup(sess.ID); ok {
		t.Fatal("expired session still live")
	}
}

func TestSessionCapRejectsNewDoesNotEvict(t *testing.T) {
	tab := NewSessionTable(SessionTableConfig{Max: 2, TTL: time.Hour})
	a := Actor{ID: "a", Role: RoleViewer}
	s1, err := tab.Create(a)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := tab.Create(a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tab.Create(a)
	if err == nil {
		t.Fatal("expected cap reject")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRateLimited || de.Message != "session table full" {
		t.Fatalf("err=%v", err)
	}
	if !de.Retryable {
		t.Fatal("rate_limited must be retryable")
	}
	if _, ok := tab.Lookup(s1.ID); !ok {
		t.Fatal("cap reject evicted s1")
	}
	if _, ok := tab.Lookup(s2.ID); !ok {
		t.Fatal("cap reject evicted s2")
	}
	if tab.Len() != 2 {
		t.Fatalf("len=%d", tab.Len())
	}
}

func TestSessionRotateDoesNotConsumeSlot(t *testing.T) {
	tab := NewSessionTable(SessionTableConfig{Max: 1, TTL: time.Hour})
	s1, err := tab.Create(Actor{ID: "v", Role: RoleViewer, Scopes: []string{ScopeDNSRead}})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := tab.Rotate(s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ID == s1.ID || s2.CSRF == s1.CSRF {
		t.Fatal("rotation reused id or csrf")
	}
	if s2.Actor.Class != ClassUISession || s2.Actor.ID != "v" || s2.Actor.Role != RoleViewer {
		t.Fatalf("rotated actor=%+v", s2.Actor)
	}
	if _, ok := tab.Lookup(s1.ID); ok {
		t.Fatal("old id still live")
	}
	if tab.Len() != 1 {
		t.Fatalf("len=%d want 1", tab.Len())
	}
	if _, err := tab.Create(Actor{ID: "other", Role: RoleAdministrator}); err == nil {
		t.Fatal("rotation left a free slot")
	}
}

func TestSessionLookupSlidesTTL(t *testing.T) {
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	cur := now
	tab := NewSessionTable(SessionTableConfig{
		TTL: time.Hour,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return cur
		},
	})
	sess, err := tab.Create(Actor{ID: "x", Role: RoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	cur = cur.Add(50 * time.Minute)
	mu.Unlock()
	if _, ok := tab.Lookup(sess.ID); !ok {
		t.Fatal("should slide")
	}
	mu.Lock()
	cur = cur.Add(50 * time.Minute)
	mu.Unlock()
	if _, ok := tab.Lookup(sess.ID); !ok {
		t.Fatal("TTL did not slide")
	}
}

func TestCSRFEqual(t *testing.T) {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if !CSRFEqual(a, a) {
		t.Fatal("equal hex")
	}
	if CSRFEqual(a, a[:len(a)-1]+"b") {
		t.Fatal("mismatch accepted")
	}
	if CSRFEqual(a, a+"00") {
		t.Fatal("length mismatch accepted")
	}
	if CSRFEqual("not-hex!", "not-hex!") {
		// non-hex equal-length still compared
	} else {
		t.Fatal("non-hex equal should compare bytes")
	}
}

func TestIdentityDigestHashesValuesNotIDs(t *testing.T) {
	p1, err := NewPolicy(PolicyConfig{
		Profile: ProfileBearer,
		Tokens:  []Token{{Token: "secret-a", ID: "one", Role: RoleViewer}, {Token: "secret-b", ID: "two"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := NewPolicy(PolicyConfig{
		Profile: ProfileBearer,
		Tokens:  []Token{{Token: "secret-a", ID: "other", Role: RoleAdministrator}, {Token: "secret-b", ID: "zz"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if IdentityDigest(p1) != IdentityDigest(p2) {
		t.Fatal("digest must ignore token ids and roles")
	}
	p3, err := NewPolicy(PolicyConfig{
		Profile: ProfileBearer,
		Tokens:  []Token{{Token: "secret-a", ID: "one"}, {Token: "secret-c", ID: "two"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if IdentityDigest(p1) == IdentityDigest(p3) {
		t.Fatal("digest must change with token values")
	}
	p4, err := NewPolicy(PolicyConfig{Profile: ProfileDevLoopbackUnauth, Tokens: p1.tokens})
	if err != nil {
		t.Fatal(err)
	}
	if IdentityDigest(p1) == IdentityDigest(p4) {
		t.Fatal("digest must include profile")
	}
}

func TestResetIfDigestChanged(t *testing.T) {
	tab := NewSessionTable(SessionTableConfig{Digest: "aaa"})
	if _, err := tab.Create(Actor{ID: "x", Role: RoleViewer}); err != nil {
		t.Fatal(err)
	}
	tab.ResetIfDigestChanged("aaa")
	if tab.Len() != 1 {
		t.Fatal("same digest reset")
	}
	tab.ResetIfDigestChanged("bbb")
	if tab.Len() != 0 {
		t.Fatal("digest change kept sessions")
	}
}

func TestClassUISessionAdministratorHasScopes(t *testing.T) {
	a := Actor{ID: "tok", Class: ClassUISession, Role: RoleAdministrator}
	if !a.HasScope(ScopeDNSAdmin) || !a.HasScope(ScopeDNSRead) || !a.HasScope(ScopeChaosEmergency) {
		t.Fatalf("effective=%v", a.EffectiveScopes())
	}
	if len(a.EffectiveScopes()) != len(AllScopes()) {
		t.Fatalf("scopes=%v", a.EffectiveScopes())
	}
}

func TestClassUISessionWithoutRoleHasNoAdmin(t *testing.T) {
	a := Actor{ID: "tok", Class: ClassUISession, Role: RoleViewer}
	if a.HasScope(ScopeDNSAdmin) {
		t.Fatal("viewer ui-session gained dns.admin")
	}
}

func TestSessionCreateConcurrent(t *testing.T) {
	tab := NewSessionTable(SessionTableConfig{Max: DefaultSessionMax, TTL: time.Hour})
	var n atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				s, err := tab.Create(Actor{ID: "c", Role: RoleViewer})
				if err != nil {
					return
				}
				if _, ok := tab.Lookup(s.ID); ok {
					n.Add(1)
				}
				if j%2 == 0 {
					rot, err := tab.Rotate(s.ID)
					if err == nil {
						tab.Delete(rot.ID)
					}
				} else {
					tab.Delete(s.ID)
				}
			}
		}()
	}
	wg.Wait()
	if n.Load() == 0 {
		t.Fatal("no successful lookups")
	}
}
