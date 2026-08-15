package snapshot

import "sync/atomic"

// Store holds the active, previous, and bootstrap snapshots behind atomic
// pointers. A DNS query loads once and retains that Snapshot for the request.
// Zero value is ready to use.
type Store struct {
	active    atomic.Pointer[Snapshot]
	previous  atomic.Pointer[Snapshot]
	bootstrap atomic.Pointer[Snapshot]
	// emergency is the runtime inhibit bit (SIGUSR1 / REST emergency-disable).
	// Swap stamps it onto every installed snapshot. Apply cannot clear it;
	// SetEmergencyChaosOff(false) can (Reset and emergency-enable).
	emergency atomic.Bool
	// startup is the --chaos-disable / LABDNS_CHAOS_DISABLE lock. It also
	// forces EmergencyChaosOff on every Swap/Stamp. Reset and
	// EmergencyEnableChaos cannot clear it; only a restart without the
	// flag/env can.
	startup atomic.Bool
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
//
// If the process emergency bit is set, Swap copies next (when needed) and
// forces EmergencyChaosOff. Apply cannot clear the inhibit this way.
func (s *Store) Swap(next *Snapshot) *Snapshot {
	next = s.stampEmergency(next)
	prev := s.active.Swap(next)
	if prev != nil {
		s.previous.Store(prev)
	}
	return prev
}

// SetEmergencyChaosOff sets the runtime inhibit bit. It does not publish a
// snapshot; call StampEmergency to copy the bit onto the current active.
// Clearing this bit does not clear a startup lock.
func (s *Store) SetEmergencyChaosOff(off bool) {
	if s == nil {
		return
	}
	s.emergency.Store(off)
}

// SetStartupChaosOff arms the process-lifetime startup inhibit. There is no
// clear: restart without --chaos-disable / LABDNS_CHAOS_DISABLE is the only off
// switch.
func (s *Store) SetStartupChaosOff() {
	if s == nil {
		return
	}
	s.startup.Store(true)
}

// StartupChaosOff reports the startup inhibit lock.
func (s *Store) StartupChaosOff() bool {
	return s != nil && s.startup.Load()
}

// EmergencyChaosOff reports whether chaos execution is inhibited by the
// runtime bit or the startup lock.
func (s *Store) EmergencyChaosOff() bool {
	return s != nil && (s.emergency.Load() || s.startup.Load())
}

// StampEmergency CAS-installs a copy of the current active snapshot with
// EmergencyChaosOff matching the process bit (OR YAML emergencyDisabled).
// It retries if a concurrent Swap won, so it never republishes a stale
// Canonical. Already-correct snapshots are left in place.
func (s *Store) StampEmergency() *Snapshot {
	if s == nil {
		return nil
	}
	for {
		live := s.active.Load()
		if live == nil {
			return nil
		}
		next := s.stampEmergency(live)
		if next == live {
			return live
		}
		next.Generation = live.Generation + 1
		if s.active.CompareAndSwap(live, next) {
			s.previous.Store(live)
			return next
		}
	}
}

func (s *Store) stampEmergency(next *Snapshot) *Snapshot {
	if s == nil || next == nil {
		return next
	}
	want := s.emergency.Load() || s.startup.Load()
	if next.Canonical != nil && next.Canonical.Spec.Chaos.EmergencyDisabled {
		want = true
	}
	if next.EmergencyChaosOff == want {
		return next
	}
	cp := *next
	cp.EmergencyChaosOff = want
	return &cp
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

// InstallBootstrap records next as bootstrap and installs it as active.
// It does not mutate next. A nil receiver or nil next is a no-op so a
// failed compile cannot clear a live store.
func (s *Store) InstallBootstrap(next *Snapshot) *Snapshot {
	if s == nil || next == nil {
		return nil
	}
	s.SetBootstrap(next)
	return s.Swap(next)
}
