package forwarder

import (
	"fmt"
	"strings"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// Compile builds an immutable ForwardingIndex from canonical state.
//
// It is fail-closed on missing pools, empty suffixes, unknown strategies,
// and empty endpoints even when config.Validate has already run. It does
// not require a fully populated Spec — Policies and Pools alone are enough
// for unit tests. Self-forward loops are a config.Validate concern (they
// need the listen address).
func Compile(st *model.State) (snapshot.ForwardingIndex, error) {
	idx := snapshot.ForwardingIndex{
		ByID:  map[model.PolicyID]*snapshot.CompiledPolicy{},
		Pools: map[model.PoolID]*snapshot.CompiledPool{},
	}
	if st == nil {
		return idx, nil
	}
	for i, p := range st.Spec.Forwarding.Pools {
		if p.ID == "" {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: pool[%d] missing id", ErrInvalidForwarding, i)
		}
		if _, dup := idx.Pools[p.ID]; dup {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: duplicate pool id %q", ErrInvalidForwarding, p.ID)
		}
		if !validStrategy(p.Strategy) {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: pool %q unknown strategy %q", ErrInvalidForwarding, p.ID, p.Strategy)
		}
		if len(p.Upstreams) == 0 {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: pool %q has no upstreams", ErrInvalidForwarding, p.ID)
		}
		ups := make([]snapshot.CompiledUpstream, 0, len(p.Upstreams))
		seenUp := map[model.UpstreamID]struct{}{}
		for j, u := range p.Upstreams {
			if u.ID == "" {
				return snapshot.ForwardingIndex{}, fmt.Errorf("%w: pool %q upstream[%d] missing id", ErrInvalidForwarding, p.ID, j)
			}
			if _, dup := seenUp[u.ID]; dup {
				return snapshot.ForwardingIndex{}, fmt.Errorf("%w: duplicate upstream id %q", ErrInvalidForwarding, u.ID)
			}
			seenUp[u.ID] = struct{}{}
			if strings.TrimSpace(u.Endpoint) == "" {
				return snapshot.ForwardingIndex{}, fmt.Errorf("%w: upstream %q missing endpoint", ErrInvalidForwarding, u.ID)
			}
			if u.Transport != model.TransportUDP && u.Transport != model.TransportTCP {
				return snapshot.ForwardingIndex{}, fmt.Errorf("%w: upstream %q transport %q", ErrInvalidForwarding, u.ID, u.Transport)
			}
			ups = append(ups, snapshot.CompiledUpstream{
				ID:        u.ID,
				Endpoint:  u.Endpoint,
				Transport: u.Transport,
			})
		}
		idx.Pools[p.ID] = &snapshot.CompiledPool{
			ID:        p.ID,
			Strategy:  p.Strategy,
			Upstreams: ups,
		}
	}
	seenSuffix := map[model.Name]model.PolicyID{}
	for i, p := range st.Spec.Forwarding.Policies {
		if p.ID == "" {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: policy[%d] missing id", ErrInvalidForwarding, i)
		}
		if _, dup := idx.ByID[p.ID]; dup {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: duplicate policy id %q", ErrInvalidForwarding, p.ID)
		}
		suf := canonicalSuffix(string(p.Suffix))
		if suf == "" {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: policy %q missing suffix", ErrInvalidForwarding, p.ID)
		}
		if prev, ok := seenSuffix[suf]; ok {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: duplicate suffix %q (%s and %s)", ErrInvalidForwarding, suf, prev, p.ID)
		}
		seenSuffix[suf] = p.ID
		if p.UpstreamPool == "" {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: policy %q missing pool", ErrInvalidForwarding, p.ID)
		}
		if _, ok := idx.Pools[p.UpstreamPool]; !ok {
			return snapshot.ForwardingIndex{}, fmt.Errorf("%w: policy %q pool %q not found", ErrInvalidForwarding, p.ID, p.UpstreamPool)
		}
		idx.ByID[p.ID] = &snapshot.CompiledPolicy{
			ID:       p.ID,
			Suffix:   suf,
			PoolID:   p.UpstreamPool,
			Failover: p.Failover,
		}
	}
	return idx, nil
}

func validStrategy(s model.PoolStrategy) bool {
	switch s {
	case model.StrategyOrdered, model.StrategyRoundRobin, model.StrategyRandom, model.StrategyHealthAware:
		return true
	default:
		return false
	}
}

func canonicalSuffix(s string) model.Name {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	if s == "." {
		return "."
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return model.Name(s)
}
