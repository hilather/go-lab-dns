package snapshot

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestForwardingSelectLongestSuffix(t *testing.T) {
	idx := ForwardingIndex{ByID: map[model.PolicyID]*CompiledPolicy{
		"corp":    {ID: "corp", Suffix: "corp.example.net."},
		"default": {ID: "default", Suffix: "."},
		"lab":     {ID: "lab", Suffix: "lab.example.net."},
	}}
	got, ok := idx.Select("a.corp.example.net.")
	if !ok || got != "corp" {
		t.Fatalf("got=%s ok=%v", got, ok)
	}
	got, ok = idx.Select("ns1.lab.example.net.")
	if !ok || got != "lab" {
		t.Fatalf("got=%s ok=%v", got, ok)
	}
	got, ok = idx.Select("other.example.")
	if !ok || got != "default" {
		t.Fatalf("default .=%s ok=%v", got, ok)
	}
}

func TestForwardingSelectEmptyAndTie(t *testing.T) {
	var empty ForwardingIndex
	if _, ok := empty.Select("a.example."); ok {
		t.Fatal("empty index selected a policy")
	}
	idx := ForwardingIndex{ByID: map[model.PolicyID]*CompiledPolicy{
		"b": {ID: "b", Suffix: "example."},
		"a": {ID: "a", Suffix: "example."},
	}}
	got, ok := idx.Select("x.example.")
	if !ok || got != "a" {
		t.Fatalf("tie should prefer smaller id, got=%s ok=%v", got, ok)
	}
}

func TestForwardingLookupAndPool(t *testing.T) {
	idx := ForwardingIndex{
		ByID: map[model.PolicyID]*CompiledPolicy{
			"p": {ID: "p", Suffix: ".", PoolID: "pool"},
		},
		Pools: map[model.PoolID]*CompiledPool{
			"pool": {ID: "pool", Strategy: model.StrategyOrdered},
		},
	}
	if _, ok := idx.Lookup(""); ok {
		t.Fatal("empty id")
	}
	if _, ok := idx.Lookup("missing"); ok {
		t.Fatal("missing policy")
	}
	p, ok := idx.Lookup("p")
	if !ok || p.PoolID != "pool" {
		t.Fatalf("lookup %+v ok=%v", p, ok)
	}
	pool, ok := idx.Pool("pool")
	if !ok || pool.ID != "pool" {
		t.Fatalf("pool %+v ok=%v", pool, ok)
	}
}
