package cache

import (
	"github.com/hilather/go-lab-dns/internal/model"
	"testing"
	"time"
)

func TestSnapshotPolicyPreventsOldCompletionRollback(t *testing.T) {
	old := RequestPolicy{Generation: 1, Policy: Policy{Enabled: true, MaxEntries: 8, MaximumTTL: time.Hour}}
	current := RequestPolicy{Generation: 2, Policy: Policy{Enabled: true, MaxEntries: 1, MaximumTTL: time.Second}}
	c := New(old.Policy, nil)
	c.ObservePolicy(old)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		<-release
		c.ObservePolicy(old)
		c.Put(Key{Revision: "old", Name: "old."}, Entry{Result: model.Result{Answers: []model.RR{{TTL: time.Hour}}}}, PutOpts{Snapshot: &old})
	}()
	<-started
	c.ObservePolicy(current)
	key := Key{Revision: "new", Name: "new."}
	c.Put(key, Entry{Result: model.Result{Answers: []model.RR{{TTL: time.Hour}}}}, PutOpts{Snapshot: &current})
	close(release)
	<-done
	if c.Policy() != current.Policy || c.Stats().Entries != 1 {
		t.Fatalf("old completion rolled back policy: %+v %+v", c.Policy(), c.Stats())
	}
	if _, ok := c.Get(Key{Revision: "old", Name: "old."}, GetOpts{Snapshot: &old}); ok {
		t.Fatal("old generation repopulated cache")
	}
	entry, ok := c.Get(key, GetOpts{Snapshot: &current})
	if !ok || entry.Result.Answers[0].TTL > time.Second {
		t.Fatalf("new snapshot TTL ignored: %+v", entry)
	}
	disabled := RequestPolicy{Generation: 3, Policy: Policy{Enabled: false, MaxEntries: 1}}
	c.ObservePolicy(disabled)
	c.ObservePolicy(old)
	if c.Enabled() || c.Stats().Entries != 0 {
		t.Fatal("disabled cache revived by old snapshot")
	}
}
