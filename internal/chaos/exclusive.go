package chaos

import (
	"sync"

	"github.com/hilather/go-lab-dns/internal/model"
)

// ExclusiveSet holds one live query's exclusive-group winners so pre- and
// post-resolution Decide share the same "only one selected policy runs"
// table. A nil set is a no-op (each Decide call then uses a private table).
type ExclusiveSet struct {
	mu      sync.Mutex
	winners map[string]model.PolicyID
}

// NewExclusiveSet returns an empty per-query exclusive-group table.
func NewExclusiveSet() *ExclusiveSet {
	return &ExclusiveSet{winners: map[string]model.PolicyID{}}
}

// Winner reports the policy that already claimed group.
func (e *ExclusiveSet) Winner(group string) (model.PolicyID, bool) {
	if e == nil || group == "" {
		return "", false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	id, ok := e.winners[group]
	return id, ok
}

// Claim records id as the winner of group if the group is still open.
func (e *ExclusiveSet) Claim(group string, id model.PolicyID) {
	if e == nil || group == "" || id == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.winners == nil {
		e.winners = map[string]model.PolicyID{}
	}
	if _, ok := e.winners[group]; ok {
		return
	}
	e.winners[group] = id
}
