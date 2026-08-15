package model

import "time"

// PoolStrategy selects an upstream from a pool.
type PoolStrategy string

const (
	StrategyOrdered     PoolStrategy = "ordered"
	StrategyRoundRobin  PoolStrategy = "round-robin"
	StrategyRandom      PoolStrategy = "random"
	StrategyHealthAware PoolStrategy = "health-aware"
)

// AllPoolStrategies is the closed first-GA upstream-pool strategy set.
var AllPoolStrategies = []PoolStrategy{
	StrategyOrdered, StrategyRoundRobin, StrategyRandom, StrategyHealthAware,
}

// ForwardingSpec holds suffix policies and upstream pools.
type ForwardingSpec struct {
	Policies []ForwardingPolicy `json:"policies"`
	Pools    []UpstreamPool     `json:"pools"`
}

// ForwardingPolicy maps a suffix to an upstream pool. Suffix "." is the default.
type ForwardingPolicy struct {
	ID           PolicyID     `json:"id"`
	Suffix       Name         `json:"suffix"`
	UpstreamPool PoolID       `json:"upstreamPool"`
	Failover     FailoverSpec `json:"failover"`
}

// FailoverSpec is explicit per-policy failover. NXDOMAIN does not fail over.
type FailoverSpec struct {
	Timeout             time.Duration `json:"timeout,omitempty"`
	OnTimeout           bool          `json:"onTimeout"`
	OnTransportError    bool          `json:"onTransportError"`
	OnSERVFAIL          bool          `json:"onSERVFAIL"`
	OnREFUSED           bool          `json:"onREFUSED"`
	UDPTruncateRetryTCP bool          `json:"udpTruncateRetryTCP"`
}

// UpstreamPool is an ordered set of upstreams plus a selection strategy.
type UpstreamPool struct {
	ID        PoolID       `json:"id"`
	Strategy  PoolStrategy `json:"strategy"`
	Upstreams []Upstream   `json:"upstreams"`
}

// Upstream is a single configured resolver. Transport is udp or tcp only.
type Upstream struct {
	ID        UpstreamID `json:"id"`
	Endpoint  string     `json:"endpoint"`
	Transport Transport  `json:"transport"`
}
