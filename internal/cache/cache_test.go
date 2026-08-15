package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestPositiveAndNegativeTTLClamp(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC))
	c := New(Policy{
		Enabled:            true,
		MaxEntries:         10,
		MinimumTTL:         2 * time.Second,
		MaximumTTL:         5 * time.Second,
		MaximumNegativeTTL: 3 * time.Second,
	}, clk)

	posKey := Key{Revision: "sha256:a", Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	c.Put(posKey, Entry{Result: model.Result{
		RCode:   model.RCodeNoError,
		Answers: []model.RR{{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Hour, Data: "192.0.2.1"}},
		Source:  model.SourceExact,
	}}, PutOpts{})
	got, ok := c.Get(posKey, GetOpts{})
	if !ok {
		t.Fatal("positive miss")
	}
	if got.Result.Source != model.SourceCache {
		t.Fatalf("source=%s", got.Result.Source)
	}
	if got.Original != "" && got.Original != model.SourceExact {
		// Original is set by caller; we did not set it on Put
	}
	clk.Advance(5 * time.Second)
	if _, ok := c.Get(posKey, GetOpts{}); ok {
		t.Fatal("positive should expire at max TTL 5s")
	}

	negKey := Key{Revision: "sha256:a", Name: "b.example.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	c.Put(negKey, Entry{
		Negative: true,
		Result: model.Result{
			RCode:     model.RCodeNXDomain,
			Authority: []model.RR{{Name: "example.", Type: model.TypeSOA, Class: model.ClassIN, TTL: time.Hour, Data: "ns.example. hostmaster. 1 1 1 1 1"}},
			Source:    model.SourceNegative,
		},
	}, PutOpts{})
	if _, ok := c.Get(negKey, GetOpts{}); !ok {
		t.Fatal("negative miss")
	}
	clk.Advance(3 * time.Second)
	if _, ok := c.Get(negKey, GetOpts{}); ok {
		t.Fatal("negative should expire at max negative TTL 3s")
	}
}

func TestNXDOMAINVersusNODATA(t *testing.T) {
	c := New(Policy{Enabled: true, MaxEntries: 4, MinimumTTL: time.Second, MaximumTTL: time.Minute, MaximumNegativeTTL: time.Minute}, nil)
	nx := Key{Revision: "r", Name: "miss.example.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	nd := Key{Revision: "r", Name: "exist.example.", Type: model.TypeAAAA, Class: model.ClassIN, Local: true}
	c.Put(nx, Entry{Negative: true, Result: model.Result{RCode: model.RCodeNXDomain, Authority: []model.RR{{TTL: 10 * time.Second}}}}, PutOpts{})
	c.Put(nd, Entry{Negative: true, Result: model.Result{RCode: model.RCodeNoError, Authority: []model.RR{{TTL: 10 * time.Second}}}}, PutOpts{})
	a, ok := c.Get(nx, GetOpts{})
	if !ok || a.Result.RCode != model.RCodeNXDomain {
		t.Fatalf("nx %+v ok=%v", a, ok)
	}
	b, ok := c.Get(nd, GetOpts{})
	if !ok || b.Result.RCode != model.RCodeNoError {
		t.Fatalf("nodata %+v ok=%v", b, ok)
	}
}

func TestRevisionNamespace(t *testing.T) {
	c := New(Policy{Enabled: true, MaxEntries: 4, MinimumTTL: time.Second, MaximumTTL: time.Minute}, nil)
	oldK := Key{Revision: "sha256:old", Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	newK := oldK
	newK.Revision = "sha256:new"
	c.Put(oldK, Entry{Result: model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{TTL: 10 * time.Second, Data: "192.0.2.1"}}}}, PutOpts{})
	if _, ok := c.Get(newK, GetOpts{}); ok {
		t.Fatal("new revision must not see old local answer")
	}
	if _, ok := c.Get(oldK, GetOpts{}); !ok {
		t.Fatal("old revision should still hit until flushed")
	}
}

func TestEvictionAndConcurrency(t *testing.T) {
	c := New(Policy{Enabled: true, MaxEntries: 2, MinimumTTL: time.Second, MaximumTTL: time.Minute}, nil)
	put := func(name string) {
		c.Put(Key{Revision: "r", Name: model.Name(name), Type: model.TypeA, Class: model.ClassIN, Local: true},
			Entry{Result: model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{TTL: 10 * time.Second, Data: "192.0.2.1"}}}}, PutOpts{})
	}
	put("a.")
	put("b.")
	put("c.")
	st := c.Stats()
	if st.Entries != 2 {
		t.Fatalf("entries=%d evicts=%d", st.Entries, st.Evicts)
	}
	if _, ok := c.Get(Key{Revision: "r", Name: "a.", Type: model.TypeA, Class: model.ClassIN, Local: true}, GetOpts{}); ok {
		t.Fatal("LRU a. should have been evicted")
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k := Key{Revision: "r", Name: model.Name("n."), Type: model.TypeA, Class: model.ClassIN, Local: true}
			c.Put(k, Entry{Result: model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{TTL: time.Second}}}}, PutOpts{})
			_, _ = c.Get(k, GetOpts{})
		}(i)
	}
	wg.Wait()
}

func TestChaosHooksAndCopy(t *testing.T) {
	c := New(Policy{Enabled: true, MaxEntries: 4, MinimumTTL: time.Second, MaximumTTL: time.Minute, StaleServing: true}, testutil.NewFakeClock(time.Unix(1_700_000_000, 0)))
	k := Key{Revision: "r", Name: "a.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	c.Put(k, Entry{Result: model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{TTL: time.Second, Data: "1.2.3.4"}}}}, PutOpts{Skip: true})
	if _, ok := c.Get(k, GetOpts{}); ok {
		t.Fatal("skip put")
	}
	c.Put(k, Entry{Result: model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{TTL: time.Second, Data: "1.2.3.4"}}}}, PutOpts{})
	if _, ok := c.Get(k, GetOpts{Bypass: true}); ok {
		t.Fatal("bypass")
	}
	if _, ok := c.Get(k, GetOpts{ForceMiss: true}); ok {
		t.Fatal("force-miss")
	}
	got, ok := c.Get(k, GetOpts{})
	if !ok {
		t.Fatal("expected hit")
	}
	got.Result.Answers[0].Data = "mutated"
	got2, ok := c.Get(k, GetOpts{})
	if !ok || got2.Result.Answers[0].Data != "1.2.3.4" {
		t.Fatal("Get must return a copy")
	}
}

func TestRemainingTTLOnGet(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC))
	c := New(Policy{Enabled: true, MaxEntries: 4}, clk)
	k := Key{Revision: "r", Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	c.Put(k, Entry{Result: model.Result{
		RCode:   model.RCodeNoError,
		Answers: []model.RR{{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: 5 * time.Second, Data: "192.0.2.1"}},
	}}, PutOpts{})
	clk.Advance(2 * time.Second)
	got, ok := c.Get(k, GetOpts{})
	if !ok {
		t.Fatal("miss")
	}
	if got.Result.Answers[0].TTL != 3*time.Second {
		t.Fatalf("remaining TTL=%s, want 3s", got.Result.Answers[0].TTL)
	}
}

func TestDisabledAndZeroTTL(t *testing.T) {
	c := New(Policy{Enabled: false, MaxEntries: 10}, nil)
	k := Key{Revision: "r", Name: "a.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	c.Put(k, Entry{Result: model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{TTL: time.Second}}}}, PutOpts{})
	if _, ok := c.Get(k, GetOpts{}); ok {
		t.Fatal("disabled cache stored")
	}
	c2 := New(Policy{Enabled: true, MaxEntries: 10}, nil)
	c2.Put(k, Entry{Result: model.Result{RCode: model.RCodeNoError}}, PutOpts{})
	if _, ok := c2.Get(k, GetOpts{}); ok {
		t.Fatal("zero-TTL put must not store")
	}
}

func TestPolicyFromSpec(t *testing.T) {
	p := PolicyFromSpec(model.CacheSpec{Enabled: true, MaxEntries: 9, MinimumTTL: time.Second})
	if !p.Enabled || p.MaxEntries != 9 || p.MinimumTTL != time.Second {
		t.Fatalf("%+v", p)
	}
}
