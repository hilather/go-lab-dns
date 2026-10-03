package snapshot

import "testing"

func TestSwapAdvancesGenerationPastEmergencyChanges(t *testing.T) {
	store := NewStore()
	original := &Snapshot{Generation: 7}
	store.Swap(original)
	candidate := &Snapshot{Generation: 8}
	store.SetEmergencyChaosOff(true)
	emergency := store.StampEmergency()
	if emergency.Generation != 8 {
		t.Fatalf("emergency generation=%d", emergency.Generation)
	}
	store.SetEmergencyChaosOff(false)
	store.StampEmergency()
	previous := store.Swap(candidate)
	published := store.Load()
	if published.Generation <= previous.Generation {
		t.Fatalf("generation regressed from %d to %d", previous.Generation, published.Generation)
	}
	if candidate.Generation != 8 {
		t.Fatal("shared compiled candidate mutated")
	}
}
