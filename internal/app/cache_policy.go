package app

import (
	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func (s *App) observeCachePolicy(snap *snapshot.Snapshot) *cache.RequestPolicy {
	if s.cache == nil || snap == nil {
		return nil
	}
	request := &cache.RequestPolicy{Generation: snap.Generation, Policy: cache.Policy(snap.CachePolicy)}
	s.cache.ObservePolicy(*request)
	return request
}
