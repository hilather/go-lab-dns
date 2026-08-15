package effects

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestCacheHooks(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC))
	c := cache.New(cache.Policy{Enabled: true, MaxEntries: 4, MinimumTTL: time.Second, MaximumTTL: time.Minute, StaleServing: true}, clk)
	key := cache.Key{Revision: "r", Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, Local: true}
	c.Put(key, cache.Entry{Result: model.Result{
		RCode:   model.RCodeNoError,
		Answers: []model.RR{{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: 30 * time.Second, Data: "192.0.2.1"}},
	}}, cache.PutOpts{})

	if _, ok := c.Get(key, CacheGet(chaos.ActionPlan{Cache: chaos.CachePlan{Bypass: true}}, nil)); ok {
		t.Fatal("bypass must miss")
	}
	if _, ok := c.Get(key, CacheGet(chaos.ActionPlan{Cache: chaos.CachePlan{ForceMiss: true}}, nil)); ok {
		t.Fatal("force-miss must miss")
	}
	if _, ok := c.Get(key, cache.GetOpts{}); !ok {
		t.Fatal("entry must remain after force-miss")
	}
	ent, ok := c.Get(key, CacheGet(chaos.ActionPlan{Cache: chaos.CachePlan{Expire: true, ServeStale: true}}, nil))
	if !ok || !ent.Stale {
		t.Fatalf("expire+stale %+v ok=%v", ent, ok)
	}
}
