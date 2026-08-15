package cache

import (
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Policy is cache bounds only. Entries live here, not in Snapshot.
type Policy struct {
	Enabled            bool
	MaxEntries         int
	MinimumTTL         time.Duration
	MaximumTTL         time.Duration
	MaximumNegativeTTL time.Duration
	StaleServing       bool
}

// Key identifies a cacheable answer. Revision namespaces local (and
// forwarded) data so a snapshot swap cannot serve a pre-mutation override.
// ForwardingID is empty for local answers; CD is only material for upstream.
type Key struct {
	Revision     model.Revision
	Name         model.Name
	Type         model.RRType
	Class        model.RRClass
	CD           bool
	ForwardingID model.PolicyID
	Local        bool
}

// Entry is one cached result plus expiry metadata.
type Entry struct {
	Result   model.Result
	StoredAt time.Time
	ExpireAt time.Time
	Negative bool
	// Original is the source before this entry was cached.
	Original model.Source
	Upstream model.UpstreamID
	Policy   model.PolicyID
	Stale    bool
}

// GetOpts are chaos/request-path hooks. Zero value is a normal lookup.
type GetOpts struct {
	Bypass       bool // skip the cache entirely
	ForceMiss    bool // pretend miss; leave the entry
	ServeStale   bool // return an expired copy when StaleServing is on
	TreatExpired bool // expire-this-request: treat the live entry as expired
}

// PutOpts are chaos/request-path hooks. Zero value stores normally.
type PutOpts struct {
	Skip bool
}

// Cache is a process-scoped bounded LRU. It is not part of Snapshot.
type Cache struct {
	mu      sync.Mutex
	policy  Policy
	clk     testutil.Clock
	entries map[Key]*node
	head    *node // MRU
	tail    *node // LRU
	hits    int
	misses  int
	evicts  int
}

type node struct {
	key  Key
	ent  Entry
	prev *node
	next *node
}

// New returns a cache using policy bounds. A disabled or MaxEntries<=0
// cache still exists but Get misses and Put is a no-op.
func New(policy Policy, clk testutil.Clock) *Cache {
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	return &Cache{
		policy:  policy,
		clk:     clk,
		entries: map[Key]*node{},
	}
}

// Policy returns the configured bounds.
func (c *Cache) Policy() Policy {
	if c == nil {
		return Policy{}
	}
	return c.policy
}

// Enabled reports whether the cache will store entries.
func (c *Cache) Enabled() bool {
	return c != nil && c.policy.Enabled && c.policy.MaxEntries > 0
}

// Get returns a copy of a live (or stale, if requested) entry.
func (c *Cache) Get(key Key, opts GetOpts) (Entry, bool) {
	if c == nil || !c.Enabled() || opts.Bypass {
		return Entry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.entries[key]
	if !ok {
		c.misses++
		return Entry{}, false
	}
	now := c.clk.Now()
	expired := !n.ent.ExpireAt.IsZero() && !now.Before(n.ent.ExpireAt)
	if opts.TreatExpired {
		expired = true
	}
	if expired {
		serveStale := opts.ServeStale || c.policy.StaleServing
		if !serveStale {
			c.removeLocked(n)
			c.misses++
			return Entry{}, false
		}
		if opts.ForceMiss {
			c.misses++
			return Entry{}, false
		}
		c.moveFrontLocked(n)
		c.hits++
		out := copyEntry(n.ent)
		out.Stale = true
		out.Result = copyResult(out.Result)
		out.Result.Source = model.SourceCache
		decayTTLs(&out.Result, now.Sub(n.ent.StoredAt), remainingTTL(n.ent, now))
		return out, true
	}
	if opts.ForceMiss {
		c.misses++
		return Entry{}, false
	}
	c.moveFrontLocked(n)
	c.hits++
	out := copyEntry(n.ent)
	out.Result = copyResult(out.Result)
	out.Result.Source = model.SourceCache
	decayTTLs(&out.Result, now.Sub(n.ent.StoredAt), remainingTTL(n.ent, now))
	return out, true
}

// Put stores a copy of ent under key, clamping TTLs and evicting LRU.
func (c *Cache) Put(key Key, ent Entry, opts PutOpts) {
	if c == nil || !c.Enabled() || opts.Skip {
		return
	}
	ttl := clampTTL(ent, c.policy)
	if ttl <= 0 {
		return
	}
	now := c.clk.Now()
	ent.StoredAt = now
	ent.ExpireAt = now.Add(ttl)
	ent.Result = copyResult(ent.Result)
	c.mu.Lock()
	defer c.mu.Unlock()
	if n, ok := c.entries[key]; ok {
		n.ent = ent
		c.moveFrontLocked(n)
		return
	}
	n := &node{key: key, ent: ent}
	c.entries[key] = n
	c.pushFrontLocked(n)
	for len(c.entries) > c.policy.MaxEntries {
		if c.tail == nil {
			break
		}
		c.removeLocked(c.tail)
		c.evicts++
	}
}

// Flush drops every entry.
func (c *Cache) Flush() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[Key]*node{}
	c.head = nil
	c.tail = nil
}

// Stats is a snapshot of counters. Hits/misses are process-local.
type Stats struct {
	Entries int
	Hits    int
	Misses  int
	Evicts  int
}

// Stats returns current sizes and counters.
func (c *Cache) Stats() Stats {
	if c == nil {
		return Stats{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{Entries: len(c.entries), Hits: c.hits, Misses: c.misses, Evicts: c.evicts}
}

// PolicyFromSpec copies cache bounds out of canonical spec.
func PolicyFromSpec(s model.CacheSpec) Policy {
	return Policy{
		Enabled:            s.Enabled,
		MaxEntries:         s.MaxEntries,
		MinimumTTL:         s.MinimumTTL,
		MaximumTTL:         s.MaximumTTL,
		MaximumNegativeTTL: s.MaximumNegativeTTL,
		StaleServing:       s.StaleServing,
	}
}

func clampTTL(ent Entry, p Policy) time.Duration {
	ttl := entryTTL(ent)
	if ent.Negative {
		if p.MaximumNegativeTTL > 0 && ttl > p.MaximumNegativeTTL {
			ttl = p.MaximumNegativeTTL
		}
		if ttl < 0 {
			return 0
		}
		return ttl
	}
	if p.MinimumTTL > 0 && ttl < p.MinimumTTL {
		ttl = p.MinimumTTL
	}
	if p.MaximumTTL > 0 && ttl > p.MaximumTTL {
		ttl = p.MaximumTTL
	}
	if ttl < 0 {
		return 0
	}
	return ttl
}

func entryTTL(ent Entry) time.Duration {
	if !ent.ExpireAt.IsZero() && !ent.StoredAt.IsZero() {
		d := ent.ExpireAt.Sub(ent.StoredAt)
		if d > 0 {
			return d
		}
	}
	var min time.Duration
	found := false
	consider := func(rrs []model.RR) {
		for _, rr := range rrs {
			if !found || rr.TTL < min {
				min = rr.TTL
				found = true
			}
		}
	}
	consider(ent.Result.Answers)
	if ent.Negative {
		consider(ent.Result.Authority)
	}
	if found {
		return min
	}
	return 0
}

func (c *Cache) pushFrontLocked(n *node) {
	n.prev = nil
	n.next = c.head
	if c.head != nil {
		c.head.prev = n
	}
	c.head = n
	if c.tail == nil {
		c.tail = n
	}
}

func (c *Cache) moveFrontLocked(n *node) {
	if n == c.head {
		return
	}
	c.unlinkLocked(n)
	c.pushFrontLocked(n)
}

func (c *Cache) removeLocked(n *node) {
	delete(c.entries, n.key)
	c.unlinkLocked(n)
}

func (c *Cache) unlinkLocked(n *node) {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
		c.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		c.tail = n.prev
	}
	n.prev = nil
	n.next = nil
}

func remainingTTL(ent Entry, now time.Time) time.Duration {
	if ent.ExpireAt.IsZero() {
		return 0
	}
	d := ent.ExpireAt.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

func decayTTLs(r *model.Result, elapsed, remaining time.Duration) {
	if r == nil {
		return
	}
	if elapsed < 0 {
		elapsed = 0
	}
	if remaining < 0 {
		remaining = 0
	}
	decay := func(rrs []model.RR) {
		for i := range rrs {
			ttl := rrs[i].TTL
			if ttl > elapsed {
				ttl -= elapsed
			} else {
				ttl = 0
			}
			// ExpireAt is the clamped lifetime; never advertise past it.
			if ttl > remaining {
				ttl = remaining
			}
			rrs[i].TTL = ttl
		}
	}
	decay(r.Answers)
	decay(r.Authority)
	decay(r.Additional)
}

func copyEntry(e Entry) Entry {
	e.Result = copyResult(e.Result)
	return e
}

func copyResult(r model.Result) model.Result {
	if r.Answers != nil {
		r.Answers = append([]model.RR(nil), r.Answers...)
	}
	if r.Authority != nil {
		r.Authority = append([]model.RR(nil), r.Authority...)
	}
	if r.Additional != nil {
		r.Additional = append([]model.RR(nil), r.Additional...)
	}
	if r.WildcardSource != nil {
		v := *r.WildcardSource
		r.WildcardSource = &v
	}
	if r.ClosestEncloser != nil {
		v := *r.ClosestEncloser
		r.ClosestEncloser = &v
	}
	if r.Explanation != nil {
		ex := *r.Explanation
		if ex.WildcardSource != nil {
			v := *ex.WildcardSource
			ex.WildcardSource = &v
		}
		if ex.ClosestEncloser != nil {
			v := *ex.ClosestEncloser
			ex.ClosestEncloser = &v
		}
		if ex.ChaosPolicyIDs != nil {
			ex.ChaosPolicyIDs = append([]model.PolicyID(nil), ex.ChaosPolicyIDs...)
		}
		if ex.ChaosActions != nil {
			ex.ChaosActions = append([]string(nil), ex.ChaosActions...)
		}
		r.Explanation = &ex
	}
	if r.EDE != nil {
		e := *r.EDE
		r.EDE = &e
	}
	return r
}
