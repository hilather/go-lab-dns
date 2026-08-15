package snapshot

import "github.com/hilather/go-lab-dns/internal/model"

// ForwardingIndex is the compiled suffix-forwarding structure. Zero value is
// valid (no policies; Select and Lookup miss).
//
// After forwarder.Compile returns, the index is immutable. Callers must not
// mutate ByID or Pools. Health bits live in forwarder runtime, not here.
type ForwardingIndex struct {
	ByID  map[model.PolicyID]*CompiledPolicy
	Pools map[model.PoolID]*CompiledPool
}

// CompiledPolicy is one suffix rule plus its failover knobs.
type CompiledPolicy struct {
	ID       model.PolicyID
	Suffix   model.Name
	PoolID   model.PoolID
	Failover model.FailoverSpec
}

// CompiledPool is one upstream pool in configured order.
type CompiledPool struct {
	ID        model.PoolID
	Strategy  model.PoolStrategy
	Upstreams []CompiledUpstream
}

// CompiledUpstream is a single configured endpoint. Transport is udp or tcp.
type CompiledUpstream struct {
	ID        model.UpstreamID
	Endpoint  string
	Transport model.Transport
}

// Lookup returns the compiled policy for id.
func (f ForwardingIndex) Lookup(id model.PolicyID) (*CompiledPolicy, bool) {
	if id == "" || f.ByID == nil {
		return nil, false
	}
	p, ok := f.ByID[id]
	return p, ok && p != nil
}

// Pool returns the compiled pool for id.
func (f ForwardingIndex) Pool(id model.PoolID) (*CompiledPool, bool) {
	if id == "" || f.Pools == nil {
		return nil, false
	}
	p, ok := f.Pools[id]
	return p, ok && p != nil
}

// Select returns the most-specific forwarding policy whose suffix matches
// qname. The root suffix "." matches every name and ranks lowest.
// Exchange must not call Select; the orchestrator passes a pre-selected ID.
func (f ForwardingIndex) Select(qname model.Name) (model.PolicyID, bool) {
	if f.ByID == nil {
		return "", false
	}
	var best model.PolicyID
	bestLen := -1
	for id, p := range f.ByID {
		if p == nil || !InZone(qname, p.Suffix) {
			continue
		}
		n := suffixRank(p.Suffix)
		if n > bestLen || (n == bestLen && string(id) < string(best)) {
			bestLen = n
			best = id
		}
	}
	return best, bestLen >= 0
}
