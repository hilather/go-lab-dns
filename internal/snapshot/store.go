package snapshot

import "sync/atomic"

// Store holds the active, previous, and bootstrap snapshots behind atomic
// pointers. A DNS query loads once and retains that Snapshot for the request.
// Zero value is ready to use.
type Store struct {
	active    atomic.Pointer[Snapshot]
	previous  atomic.Pointer[Snapshot]
	bootstrap atomic.Pointer[Snapshot]
}

// NewStore returns an empty Store. Load, Previous, and Bootstrap are nil.
func NewStore() *Store {
	return &Store{}
}

// Load returns the active snapshot, or nil if none has been installed.
func (s *Store) Load() *Snapshot {
	return s.active.Load()
}

// Swap installs next as the active snapshot and returns the previous active
// pointer. A non-nil displaced snapshot becomes Previous, replacing any
// older generation. Swap does not change Bootstrap.
func (s *Store) Swap(next *Snapshot) *Snapshot {
	prev := s.active.Swap(next)
	if prev != nil {
		s.previous.Store(prev)
	}
	return prev
}

// Bootstrap returns the compiled bootstrap snapshot, or nil.
func (s *Store) Bootstrap() *Snapshot {
	return s.bootstrap.Load()
}

// Previous returns the last non-nil snapshot displaced by Swap, or nil.
func (s *Store) Previous() *Snapshot {
	return s.previous.Load()
}

// SetBootstrap records the compiled bootstrap snapshot without changing
// active or previous.
func (s *Store) SetBootstrap(next *Snapshot) {
	s.bootstrap.Store(next)
}
