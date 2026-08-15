package snapshot

import (
	"sync"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestStoreZeroLoadPreviousBootstrapNil(t *testing.T) {
	s := NewStore()
	if s.Load() != nil || s.Previous() != nil || s.Bootstrap() != nil {
		t.Fatalf("empty store has pointers: load=%p prev=%p boot=%p", s.Load(), s.Previous(), s.Bootstrap())
	}
}

func TestStoreSwapPreviousAndBootstrap(t *testing.T) {
	s := NewStore()
	boot := &Snapshot{Generation: 0, Revision: "sha256:boot"}
	a := &Snapshot{Generation: 1, Revision: "sha256:a"}
	b := &Snapshot{Generation: 2, Revision: "sha256:b"}
	c := &Snapshot{Generation: 3, Revision: "sha256:c"}

	s.SetBootstrap(boot)
	if s.Bootstrap() != boot {
		t.Fatal("SetBootstrap did not stick")
	}
	if s.Load() != nil {
		t.Fatal("SetBootstrap must not change active")
	}

	if prev := s.Swap(a); prev != nil {
		t.Fatalf("first Swap previous = %p, want nil", prev)
	}
	if s.Load() != a {
		t.Fatal("Load after first Swap")
	}
	if s.Previous() != nil {
		t.Fatal("Previous after first Swap should be nil")
	}
	if s.Bootstrap() != boot {
		t.Fatal("Swap must not change Bootstrap")
	}

	if prev := s.Swap(b); prev != a {
		t.Fatalf("second Swap returned %p, want a", prev)
	}
	if s.Load() != b || s.Previous() != a {
		t.Fatal("after second Swap")
	}

	if prev := s.Swap(c); prev != b {
		t.Fatalf("third Swap returned %p, want b", prev)
	}
	if s.Load() != c {
		t.Fatal("Load after third Swap")
	}
	if s.Previous() != b {
		t.Fatalf("Previous must be only the last generation, got %p", s.Previous())
	}
	if s.Bootstrap() != boot {
		t.Fatal("bootstrap lost")
	}
}

func TestStoreSwapNilActive(t *testing.T) {
	s := NewStore()
	a := &Snapshot{Generation: 1}
	s.Swap(a)
	if prev := s.Swap(nil); prev != a {
		t.Fatalf("Swap(nil) returned %p", prev)
	}
	if s.Load() != nil {
		t.Fatal("active should be nil")
	}
	if s.Previous() != a {
		t.Fatal("Previous keeps last non-nil displaced snapshot after Swap(nil)")
	}
	c := &Snapshot{Generation: 3}
	if prev := s.Swap(c); prev != nil {
		t.Fatalf("Swap(C) after nil active returned %p, want nil", prev)
	}
	if s.Load() != c || s.Previous() != a {
		t.Fatal("Previous must stay A after Swap(nil) then Swap(C)")
	}
}

func TestStoreInstallBootstrap(t *testing.T) {
	s := NewStore()
	boot := &Snapshot{Generation: 0, Revision: "sha256:boot"}
	if prev := s.InstallBootstrap(boot); prev != nil {
		t.Fatalf("first InstallBootstrap previous = %p", prev)
	}
	if s.Bootstrap() != boot || s.Load() != boot {
		t.Fatal("InstallBootstrap must set both bootstrap and active")
	}
	if s.Previous() != nil {
		t.Fatal("first install must not invent a previous generation")
	}
	next := &Snapshot{Generation: 1, Revision: "sha256:next"}
	if prev := s.Swap(next); prev != boot {
		t.Fatalf("Swap after install returned %p", prev)
	}
	if s.Bootstrap() != boot || s.Load() != next || s.Previous() != boot {
		t.Fatal("bootstrap must stay the installed snapshot after Swap")
	}
}

func TestStoreInstallBootstrapNilNoOp(t *testing.T) {
	s := NewStore()
	live := &Snapshot{Generation: 1}
	s.Swap(live)
	if prev := s.InstallBootstrap(nil); prev != nil {
		t.Fatalf("InstallBootstrap(nil) returned %p", prev)
	}
	if s.Load() != live || s.Bootstrap() != nil {
		t.Fatal("nil install must not clear active or invent a bootstrap")
	}
	var none *Store
	if prev := none.InstallBootstrap(live); prev != nil {
		t.Fatalf("nil store returned %p", prev)
	}
}

func TestStoreSetBootstrapIndependent(t *testing.T) {
	s := NewStore()
	boot1 := &Snapshot{Generation: 0}
	boot2 := &Snapshot{Generation: 10}
	live := &Snapshot{Generation: 1}
	s.SetBootstrap(boot1)
	s.Swap(live)
	s.SetBootstrap(boot2)
	if s.Bootstrap() != boot2 {
		t.Fatal("bootstrap not replaced")
	}
	if s.Load() != live {
		t.Fatal("active changed by SetBootstrap")
	}
}

func TestStoreConcurrentLoadSwapPreviousBootstrap(t *testing.T) {
	s := NewStore()
	boot := &Snapshot{Generation: 0, Canonical: &model.State{Kind: model.KindLabDNS}}
	s.SetBootstrap(boot)

	snaps := make([]*Snapshot, 32)
	for i := range snaps {
		snaps[i] = &Snapshot{Generation: model.Generation(i + 1)}
	}

	ctx := testutil.Context(t)
	rng := testutil.NewSeededRand(42)
	var wg sync.WaitGroup
	const workers = 16
	const iters = 200
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				if ctx.Err() != nil {
					return
				}
				switch rng.Uint64() % 5 {
				case 0:
					_ = s.Load()
				case 1:
					_ = s.Swap(snaps[int(rng.Uint64()%uint64(len(snaps)))])
				case 2:
					_ = s.Previous()
				case 3:
					_ = s.Bootstrap()
				default:
					s.SetBootstrap(boot)
				}
			}
		}()
	}
	wg.Wait()

	if s.Bootstrap() == nil {
		t.Fatal("bootstrap pointer lost after concurrent ops")
	}
	// Active may be nil only if a Swap(nil) happened; we never swapped nil.
	if got := s.Load(); got != nil {
		if got.Generation == 0 {
			t.Fatal("active is the bootstrap object; Swap should install distinct snapshots")
		}
	}
}

func TestStoreSwapStampsEmergencyAndApplyCannotClear(t *testing.T) {
	s := NewStore()
	boot := &Snapshot{Generation: 0, Revision: "sha256:boot", Canonical: &model.State{Kind: model.KindLabDNS}}
	s.InstallBootstrap(boot)
	if s.EmergencyChaosOff() || s.Load().EmergencyChaosOff {
		t.Fatal("inhibit set at bootstrap")
	}

	s.SetEmergencyChaosOff(true)
	if !s.EmergencyChaosOff() {
		t.Fatal("SetEmergencyChaosOff did not stick")
	}
	stamped := s.StampEmergency()
	if stamped == boot {
		t.Fatal("stamp must publish a new snapshot pointer")
	}
	if !stamped.EmergencyChaosOff || stamped.Revision != boot.Revision {
		t.Fatalf("stamped off=%v rev=%s", stamped.EmergencyChaosOff, stamped.Revision)
	}
	if stamped.Canonical != boot.Canonical {
		t.Fatal("stamp must keep the same Canonical pointer")
	}

	applied := &Snapshot{Generation: 2, Revision: "sha256:applied", Canonical: &model.State{Kind: model.KindLabDNS, Metadata: model.Metadata{Name: "applied"}}}
	if prev := s.Swap(applied); prev != stamped {
		t.Fatalf("swap prev=%p want stamped", prev)
	}
	live := s.Load()
	if live.Revision != "sha256:applied" {
		t.Fatal("swap lost the applied Canonical")
	}
	if !live.EmergencyChaosOff {
		t.Fatal("swap must stamp inhibit onto apply")
	}
	if live == applied {
		t.Fatal("swap must copy when forcing the inhibit bit")
	}

	s.SetEmergencyChaosOff(false)
	cleared := s.StampEmergency()
	if cleared.EmergencyChaosOff {
		t.Fatal("enable must allow clearing the snapshot bit")
	}
}

func TestStoreStampEmergencyDoesNotRepublishStaleCanonical(t *testing.T) {
	s := NewStore()
	old := &Snapshot{Generation: 1, Revision: "sha256:old", Canonical: &model.State{Kind: model.KindLabDNS, Metadata: model.Metadata{Name: "old"}}}
	neu := &Snapshot{Generation: 2, Revision: "sha256:new", Canonical: &model.State{Kind: model.KindLabDNS, Metadata: model.Metadata{Name: "new"}}}
	s.Swap(old)
	s.SetEmergencyChaosOff(true)
	s.Swap(neu)
	got := s.StampEmergency()
	if got.Revision != "sha256:new" || got.Canonical.Metadata.Name != "new" {
		t.Fatalf("stamp republished stale Canonical rev=%s name=%s", got.Revision, got.Canonical.Metadata.Name)
	}
	if !got.EmergencyChaosOff {
		t.Fatal("stamp lost inhibit on the new snapshot")
	}
}

func TestZeroValueSnapshotIndexes(t *testing.T) {
	var snap Snapshot
	_ = snap.Zones
	_ = snap.Forwarding
	_ = snap.Chaos
	_ = snap.Access
	if snap.EmergencyChaosOff {
		t.Fatal("zero Snapshot should not inhibit via EmergencyChaosOff")
	}
}
