package app

import (
	"context"
	"strconv"
	"sync"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// auditRing is a bounded in-memory log. Oldest events fall off the front.
type auditRing struct {
	mu     sync.Mutex
	max    int
	nextID uint64
	events []AuditEvent
}

func newAuditRing(max int) *auditRing {
	if max <= 0 {
		max = defaultAuditMax
	}
	return &auditRing{max: max, events: make([]AuditEvent, 0, max)}
}

func (r *auditRing) append(ev AuditEvent) string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	ev.ID = "aud-" + strconv.FormatUint(r.nextID, 10)
	r.events = append(r.events, ev)
	if len(r.events) > r.max {
		r.events = append([]AuditEvent(nil), r.events[len(r.events)-r.max:]...)
	}
	return ev.ID
}

func (r *auditRing) list(limit int) []AuditEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > defaultPageLimit {
		limit = defaultPageLimit
	}
	n := len(r.events)
	if limit > n {
		limit = n
	}
	out := make([]AuditEvent, limit)
	// Newest first.
	for i := 0; i < limit; i++ {
		out[i] = r.events[n-1-i]
	}
	return out
}

func (r *auditRing) get(id string) (AuditEvent, bool) {
	if r == nil || id == "" {
		return AuditEvent{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].ID == id {
			return r.events[i], true
		}
	}
	return AuditEvent{}, false
}

func (s *App) QueryAudit(ctx context.Context, actor Actor, in AuditQuery) (*AuditList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	return &AuditList{Events: s.audit.list(in.Limit)}, nil
}

func (s *App) GetAudit(ctx context.Context, actor Actor, id string) (*AuditEvent, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	ev, ok := s.audit.get(id)
	if !ok {
		return nil, domainerr.NotFound("audit event " + id + " not found")
	}
	return &ev, nil
}
