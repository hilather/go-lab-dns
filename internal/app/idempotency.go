package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

type idempEntry struct {
	key   string
	fp    string
	plan  *Plan
	apply *ApplyResult
	prev  *idempEntry
	next  *idempEntry
}

// idempCache is a process-local LRU. Reset clears it. Zero max is rejected
// by New so this cannot become unbounded.
type idempCache struct {
	mu      sync.Mutex
	max     int
	entries map[string]*idempEntry
	head    *idempEntry
	tail    *idempEntry
}

func newIdempCache(max int) *idempCache {
	if max <= 0 {
		max = defaultIdempotencyMax
	}
	return &idempCache{
		max:     max,
		entries: map[string]*idempEntry{},
	}
}

func (c *idempCache) lookup(key, fp string) (*idempEntry, error) {
	if c == nil || key == "" {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, nil
	}
	if e.fp != fp {
		return nil, domainerr.IdempotencyConflict("idempotency key reused with a different request")
	}
	c.moveFrontLocked(e)
	return e, nil
}

func (c *idempCache) storePlan(key, fp string, p *Plan) {
	if c == nil || key == "" || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok && e.fp == fp {
		e.plan = clonePlan(p)
		c.moveFrontLocked(e)
		return
	}
	c.insertFrontLocked(&idempEntry{key: key, fp: fp, plan: clonePlan(p)})
}

func (c *idempCache) storeApply(key, fp string, r *ApplyResult) {
	if c == nil || key == "" || r == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok && e.fp == fp {
		e.apply = cloneApply(r)
		c.moveFrontLocked(e)
		return
	}
	c.insertFrontLocked(&idempEntry{key: key, fp: fp, apply: cloneApply(r)})
}

func (c *idempCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]*idempEntry{}
	c.head = nil
	c.tail = nil
}

func (c *idempCache) insertFrontLocked(e *idempEntry) {
	if old, ok := c.entries[e.key]; ok {
		c.removeLocked(old)
	}
	c.entries[e.key] = e
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
	for len(c.entries) > c.max && c.tail != nil {
		c.removeLocked(c.tail)
	}
}

func (c *idempCache) moveFrontLocked(e *idempEntry) {
	if e == nil || e == c.head {
		return
	}
	c.unlinkLocked(e)
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

func (c *idempCache) removeLocked(e *idempEntry) {
	if e == nil {
		return
	}
	delete(c.entries, e.key)
	c.unlinkLocked(e)
}

func (c *idempCache) unlinkLocked(e *idempEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

type changeFingerprint struct {
	ExpectedRevision model.Revision    `json:"expectedRevision"`
	Reason           string            `json:"reason"`
	Ticket           string            `json:"ticket"`
	Operations       []model.Operation `json:"operations"`
}

func fingerprintChange(in ChangeIn) (string, error) {
	ops := make([]model.Operation, len(in.Operations))
	for i, op := range in.Operations {
		ops[i] = op
		if len(op.Value) > 0 {
			var compact json.RawMessage
			if err := json.Unmarshal(op.Value, &compact); err == nil {
				if b, err := json.Marshal(compact); err == nil {
					ops[i].Value = b
				}
			}
		}
	}
	b, err := json.Marshal(changeFingerprint{
		ExpectedRevision: in.ExpectedRevision,
		Reason:           in.Reason,
		Ticket:           in.Ticket,
		Operations:       ops,
	})
	if err != nil {
		return "", domainerr.Internal("idempotency fingerprint: " + err.Error())
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func clonePlan(p *Plan) *Plan {
	if p == nil {
		return nil
	}
	out := *p
	if p.Diff != nil {
		out.Diff = append([]DiffEntry(nil), p.Diff...)
	}
	if p.Warnings != nil {
		out.Warnings = append([]Warning(nil), p.Warnings...)
	}
	if p.Operations != nil {
		out.Operations = append([]model.Operation(nil), p.Operations...)
	}
	out.Impact = cloneImpact(p.Impact)
	out.Auth.Scopes = append([]string(nil), p.Auth.Scopes...)
	return &out
}

func cloneApply(r *ApplyResult) *ApplyResult {
	if r == nil {
		return nil
	}
	out := *r
	out.Plan = *clonePlan(&r.Plan)
	return &out
}

func cloneImpact(in Impact) Impact {
	out := in
	out.Names = append([]model.Name(nil), in.Names...)
	out.Zones = append([]model.ZoneID(nil), in.Zones...)
	out.ClientGroups = append([]model.ClientGroupID(nil), in.ClientGroups...)
	out.ChaosPolicies = append([]ChaosImpact(nil), in.ChaosPolicies...)
	out.CompatibilityWarnings = append([]string(nil), in.CompatibilityWarnings...)
	out.RequiredPermissions = append([]string(nil), in.RequiredPermissions...)
	out.SuggestedProbes = append([]string(nil), in.SuggestedProbes...)
	return out
}
