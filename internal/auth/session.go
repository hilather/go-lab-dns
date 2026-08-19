package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

const (
	// CookieName is the host-only session cookie.
	CookieName = "labdns_session"
	// CSRFHeader is required on cookie-authenticated non-GET requests.
	CSRFHeader = "X-LabDNS-CSRF"
	// ClassUISession is the Actor.Class for browser sessions.
	ClassUISession = "ui-session"
	// DefaultSessionTTL is the sliding lifetime of a live session.
	DefaultSessionTTL = 12 * time.Hour
	// DefaultSessionMax is the in-process session table cap.
	DefaultSessionMax = 256

	sessionIDBytes = 32
)

// Session is one in-process browser session.
type Session struct {
	ID        string
	CSRF      string
	Actor     Actor
	ExpiresAt time.Time
}

// SessionTableConfig constructs a SessionTable.
type SessionTableConfig struct {
	// Digest is the identity digest at construction (profile + hashed token values).
	Digest string
	// TTL is the sliding lifetime. Non-positive uses DefaultSessionTTL.
	TTL time.Duration
	// Max is the distinct-session cap. Non-positive uses DefaultSessionMax.
	Max int
	// Now overrides the clock. Nil uses time.Now.
	Now func() time.Time
}

// SessionTable is the in-process session store (ADR 0003).
type SessionTable struct {
	mu     sync.Mutex
	byID   map[string]*Session
	digest string
	ttl    time.Duration
	max    int
	now    func() time.Time
}

// NewSessionTable builds an empty table.
func NewSessionTable(cfg SessionTableConfig) *SessionTable {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	max := cfg.Max
	if max <= 0 {
		max = DefaultSessionMax
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &SessionTable{
		byID:   map[string]*Session{},
		digest: cfg.Digest,
		ttl:    ttl,
		max:    max,
		now:    now,
	}
}

// IdentityDigest is SHA-256(profile || 0x00 || SHA-256(token value)… in store order).
// Token values are hashed in memory and must never be logged. Nil policy is the empty digest.
func IdentityDigest(p *Policy) string {
	h := sha256.New()
	if p != nil {
		h.Write([]byte(p.Profile()))
		h.Write([]byte{0})
		for _, t := range p.tokens {
			sum := sha256.Sum256([]byte(t.Token))
			h.Write(sum[:])
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ResetIfDigestChanged drops every session when digest differs.
// 1.1.0 never calls this; a later token-reread must call it on change.
func (t *SessionTable) ResetIfDigestChanged(digest string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if digest == t.digest {
		return
	}
	t.byID = map[string]*Session{}
	t.digest = digest
}

// Create inserts a new session for actor (Class forced to ui-session).
// At cap, expired rows are swept; a still-full table rejects with rate_limited.
func (t *SessionTable) Create(actor Actor) (*Session, error) {
	if t == nil {
		return nil, domainerr.Internal("session table is not configured")
	}
	id, err := randomHex()
	if err != nil {
		return nil, domainerr.Internal("session id entropy failed")
	}
	csrf, err := randomHex()
	if err != nil {
		return nil, domainerr.Internal("csrf entropy failed")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.expireLocked()
	if len(t.byID) >= t.max {
		return nil, domainerr.RateLimited("session table full")
	}
	now := t.now()
	sess := &Session{
		ID:        id,
		CSRF:      csrf,
		Actor:     sessionActor(actor),
		ExpiresAt: now.Add(t.ttl),
	}
	t.byID[id] = sess
	return copySession(sess), nil
}

// Rotate replaces oldID with a new ID and CSRF for the same Actor.
// It does not consume an extra table slot.
func (t *SessionTable) Rotate(oldID string) (*Session, error) {
	if t == nil {
		return nil, domainerr.Internal("session table is not configured")
	}
	id, err := randomHex()
	if err != nil {
		return nil, domainerr.Internal("session id entropy failed")
	}
	csrf, err := randomHex()
	if err != nil {
		return nil, domainerr.Internal("csrf entropy failed")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	old, ok := t.byID[oldID]
	if !ok || !old.ExpiresAt.After(t.now()) {
		delete(t.byID, oldID)
		return nil, domainerr.Unauthenticated("session expired")
	}
	delete(t.byID, oldID)
	now := t.now()
	sess := &Session{
		ID:        id,
		CSRF:      csrf,
		Actor:     sessionActor(old.Actor),
		ExpiresAt: now.Add(t.ttl),
	}
	t.byID[id] = sess
	return copySession(sess), nil
}

// Lookup returns a live session and slides its TTL. Expired IDs are dropped.
func (t *SessionTable) Lookup(id string) (*Session, bool) {
	if t == nil || id == "" {
		return nil, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	sess, ok := t.byID[id]
	if !ok {
		return nil, false
	}
	now := t.now()
	if !sess.ExpiresAt.After(now) {
		delete(t.byID, id)
		return nil, false
	}
	sess.ExpiresAt = now.Add(t.ttl)
	return copySession(sess), true
}

// Delete drops a session if present.
func (t *SessionTable) Delete(id string) {
	if t == nil || id == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byID, id)
}

// Len is the number of stored rows including not-yet-swept expired ones.
func (t *SessionTable) Len() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.expireLocked()
	return len(t.byID)
}

// CSRFEqual compares secrets in constant time.
func CSRFEqual(got, want string) bool {
	gb, errG := hex.DecodeString(got)
	wb, errW := hex.DecodeString(want)
	if errG != nil || errW != nil || len(gb) != len(wb) {
		if len(got) != len(want) {
			return false
		}
		return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
	}
	return subtle.ConstantTimeCompare(gb, wb) == 1
}

func (t *SessionTable) expireLocked() {
	now := t.now()
	for id, sess := range t.byID {
		if !sess.ExpiresAt.After(now) {
			delete(t.byID, id)
		}
	}
}

func sessionActor(a Actor) Actor {
	return Actor{
		ID:     a.ID,
		Class:  ClassUISession,
		Role:   a.Role,
		Scopes: append([]string(nil), a.Scopes...),
		Groups: append([]string(nil), a.Groups...),
	}
}

func copySession(s *Session) *Session {
	if s == nil {
		return nil
	}
	out := *s
	out.Actor = sessionActor(s.Actor)
	return &out
}

func randomHex() (string, error) {
	var b [sessionIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
