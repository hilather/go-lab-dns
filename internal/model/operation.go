package model

import "encoding/json"

// OpKind is a typed change-set verb.
type OpKind string

const (
	OpAdd    OpKind = "add"
	OpUpdate OpKind = "update"
	OpRemove OpKind = "remove"
)

// AllOpKinds is the closed first-GA operation set.
var AllOpKinds = []OpKind{OpAdd, OpUpdate, OpRemove}

// TargetKind names the object an Operation mutates.
type TargetKind string

const (
	TargetZone             TargetKind = "zone"
	TargetRecord           TargetKind = "record"
	TargetForwardingPolicy TargetKind = "forwardingPolicy"
	TargetUpstreamPool     TargetKind = "upstreamPool"
	TargetUpstream         TargetKind = "upstream"
	TargetClientGroup      TargetKind = "clientGroup"
	TargetChaosPolicy      TargetKind = "chaosPolicy"
	TargetChaosSafety      TargetKind = "chaosSafety"
	TargetCache            TargetKind = "cache"
	TargetDefaults         TargetKind = "defaults"
	TargetListeners        TargetKind = "listeners"
	TargetAccess           TargetKind = "access"
	TargetObservability    TargetKind = "observability"
	TargetManagement       TargetKind = "management"
	TargetChaosActivation  TargetKind = "chaosActivation"
)

// AllTargetKinds is the closed first-GA target set. Unknown kinds fail validation later.
var AllTargetKinds = []TargetKind{
	TargetZone,
	TargetRecord,
	TargetForwardingPolicy,
	TargetUpstreamPool,
	TargetUpstream,
	TargetClientGroup,
	TargetChaosPolicy,
	TargetChaosSafety,
	TargetCache,
	TargetDefaults,
	TargetListeners,
	TargetAccess,
	TargetObservability,
	TargetManagement,
	TargetChaosActivation,
}

// Operation is one typed change. Value is required for add/update and is
// decoded against Target.Kind by the application layer.
type Operation struct {
	Op     OpKind          `json:"op"`
	Target Target          `json:"target"`
	Value  json.RawMessage `json:"value,omitempty"`
}

// Target identifies the object an Operation applies to.
type Target struct {
	Kind   TargetKind `json:"kind"`
	ID     string     `json:"id,omitempty"`
	ZoneID ZoneID     `json:"zoneId,omitempty"`
}
