package testutil

import "testing"

func TestSeededRandDeterministic(t *testing.T) {
	a := NewSeededRand(42)
	b := NewSeededRand(42)
	c := NewSeededRand(43)
	const n = 16
	seqA := make([]uint64, n)
	seqB := make([]uint64, n)
	seqC := make([]uint64, n)
	for i := 0; i < n; i++ {
		seqA[i] = a.Uint64()
		seqB[i] = b.Uint64()
		seqC[i] = c.Uint64()
	}
	sameAB, sameAC := true, true
	for i := 0; i < n; i++ {
		if seqA[i] != seqB[i] {
			sameAB = false
		}
		if seqA[i] != seqC[i] {
			sameAC = false
		}
	}
	if !sameAB {
		t.Fatalf("same seed diverged: %v vs %v", seqA, seqB)
	}
	if sameAC {
		t.Fatalf("different seeds produced the same sequence: %v", seqA)
	}
}
