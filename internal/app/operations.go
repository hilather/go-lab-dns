package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func applyOperations(st *model.State, ops []model.Operation) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: "required", Message: "state is nil"})
	}
	for i, op := range ops {
		if err := applyOne(st, op, i); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(st *model.State, op model.Operation, i int) error {
	path := "operations[" + strconv.Itoa(i) + "]"
	switch op.Op {
	case model.OpAdd, model.OpUpdate, model.OpRemove:
	default:
		return domainerr.ValidationFailed("unknown operation",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "op must be add, update, or remove"})
	}
	switch op.Target.Kind {
	case model.TargetZone:
		return applyZone(st, op, path)
	case model.TargetRecord:
		return applyRecord(st, op, path)
	case model.TargetForwardingPolicy:
		return applyFwdPolicy(st, op, path)
	case model.TargetUpstreamPool:
		return applyPool(st, op, path)
	case model.TargetUpstream:
		return applyUpstream(st, op, path)
	case model.TargetClientGroup:
		return applyClientGroup(st, op, path)
	case model.TargetChaosPolicy:
		return applyChaosPolicy(st, op, path)
	case model.TargetChaosSafety:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.SafetySpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Chaos.Safety = v
			return nil
		})
	case model.TargetCache:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.CacheSpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Cache = v
			return nil
		})
	case model.TargetDefaults:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.DefaultsSpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Defaults = v
			return nil
		})
	case model.TargetListeners:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.ListenersSpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Listeners = v
			return nil
		})
	case model.TargetAccess:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.AccessSpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Access = v
			return nil
		})
	case model.TargetObservability:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.ObservabilitySpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Observability = v
			return nil
		})
	case model.TargetManagement:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.ManagementSpec
			if err := decodeValue(raw, &v, path+".value"); err != nil {
				return err
			}
			st.Spec.Management = v
			return nil
		})
	case model.TargetUI:
		return applySingleton(op, path, true, func(raw json.RawMessage) error {
			var v model.UISpec
			if err := decodeValue(injectUIEnabled(raw), &v, path+".value"); err != nil {
				return err
			}
			st.Spec.UI = v
			return nil
		})
	case model.TargetChaosActivation:
		return applyChaosActivation(st, op, path)
	default:
		return domainerr.ValidationFailed("unknown target kind",
			domainerr.FieldViolation{Path: path + ".target.kind", Code: "invalid_value", Message: "unknown target kind"})
	}
}

func applySingleton(op model.Operation, path string, updateOnly bool, set func(json.RawMessage) error) error {
	if updateOnly && op.Op != model.OpUpdate {
		return domainerr.ValidationFailed("singleton is update-only",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "target allows only update"})
	}
	if op.Op == model.OpRemove {
		return domainerr.ValidationFailed("cannot remove singleton",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "target cannot be removed"})
	}
	if len(bytes.TrimSpace(op.Value)) == 0 {
		return domainerr.ValidationFailed("missing value",
			domainerr.FieldViolation{Path: path + ".value", Code: "required", Message: "value is required"})
	}
	return set(op.Value)
}

func applyZone(st *model.State, op model.Operation, path string) error {
	id := model.ZoneID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	idx := -1
	for i := range st.Spec.Zones {
		if st.Spec.Zones[i].ID == id {
			idx = i
			break
		}
	}
	switch op.Op {
	case model.OpAdd:
		if idx >= 0 {
			return domainerr.AlreadyExists("zone " + string(id) + " already exists")
		}
		var z model.Zone
		if err := decodeValue(op.Value, &z, path+".value"); err != nil {
			return err
		}
		if z.ID == "" {
			z.ID = id
		}
		if z.ID != id {
			return idMismatch(path, string(id), string(z.ID))
		}
		st.Spec.Zones = append(st.Spec.Zones, z)
	case model.OpUpdate:
		if idx < 0 {
			return domainerr.NotFound("zone " + string(id) + " not found")
		}
		var z model.Zone
		if err := decodeValue(op.Value, &z, path+".value"); err != nil {
			return err
		}
		if z.ID == "" {
			z.ID = id
		}
		if z.ID != id {
			return idMismatch(path, string(id), string(z.ID))
		}
		st.Spec.Zones[idx] = z
	case model.OpRemove:
		if idx < 0 {
			return domainerr.NotFound("zone " + string(id) + " not found")
		}
		st.Spec.Zones = append(st.Spec.Zones[:idx], st.Spec.Zones[idx+1:]...)
	}
	return nil
}

func applyRecord(st *model.State, op model.Operation, path string) error {
	if op.Target.ZoneID == "" {
		return domainerr.ValidationFailed("zoneId is required for record targets",
			domainerr.FieldViolation{Path: path + ".target.zoneId", Code: "required", Message: "zoneId is required when kind is record"})
	}
	id := model.RecordID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	zi := -1
	for i := range st.Spec.Zones {
		if st.Spec.Zones[i].ID == op.Target.ZoneID {
			zi = i
			break
		}
	}
	if zi < 0 {
		return domainerr.NotFound("zone " + string(op.Target.ZoneID) + " not found")
	}
	recs := st.Spec.Zones[zi].Records
	ri := -1
	for i := range recs {
		if recs[i].ID == id {
			ri = i
			break
		}
	}
	switch op.Op {
	case model.OpAdd:
		if ri >= 0 {
			return domainerr.AlreadyExists("record " + string(id) + " already exists")
		}
		var r model.Record
		if err := decodeValue(op.Value, &r, path+".value"); err != nil {
			return err
		}
		if r.ID == "" {
			r.ID = id
		}
		if r.ID != id {
			return idMismatch(path, string(id), string(r.ID))
		}
		st.Spec.Zones[zi].Records = append(st.Spec.Zones[zi].Records, r)
	case model.OpUpdate:
		if ri < 0 {
			return domainerr.NotFound("record " + string(id) + " not found")
		}
		var r model.Record
		if err := decodeValue(op.Value, &r, path+".value"); err != nil {
			return err
		}
		if r.ID == "" {
			r.ID = id
		}
		if r.ID != id {
			return idMismatch(path, string(id), string(r.ID))
		}
		st.Spec.Zones[zi].Records[ri] = r
	case model.OpRemove:
		if ri < 0 {
			return domainerr.NotFound("record " + string(id) + " not found")
		}
		st.Spec.Zones[zi].Records = append(recs[:ri], recs[ri+1:]...)
	}
	return nil
}

func applyFwdPolicy(st *model.State, op model.Operation, path string) error {
	id := model.PolicyID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	idx := -1
	for i := range st.Spec.Forwarding.Policies {
		if st.Spec.Forwarding.Policies[i].ID == id {
			idx = i
			break
		}
	}
	switch op.Op {
	case model.OpAdd:
		if idx >= 0 {
			return domainerr.AlreadyExists("forwarding policy " + string(id) + " already exists")
		}
		var p model.ForwardingPolicy
		if err := decodeValue(op.Value, &p, path+".value"); err != nil {
			return err
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			return idMismatch(path, string(id), string(p.ID))
		}
		st.Spec.Forwarding.Policies = append(st.Spec.Forwarding.Policies, p)
	case model.OpUpdate:
		if idx < 0 {
			return domainerr.NotFound("forwarding policy " + string(id) + " not found")
		}
		var p model.ForwardingPolicy
		if err := decodeValue(op.Value, &p, path+".value"); err != nil {
			return err
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			return idMismatch(path, string(id), string(p.ID))
		}
		st.Spec.Forwarding.Policies[idx] = p
	case model.OpRemove:
		if idx < 0 {
			return domainerr.NotFound("forwarding policy " + string(id) + " not found")
		}
		st.Spec.Forwarding.Policies = append(st.Spec.Forwarding.Policies[:idx], st.Spec.Forwarding.Policies[idx+1:]...)
	}
	return nil
}

func applyPool(st *model.State, op model.Operation, path string) error {
	id := model.PoolID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	idx := -1
	for i := range st.Spec.Forwarding.Pools {
		if st.Spec.Forwarding.Pools[i].ID == id {
			idx = i
			break
		}
	}
	switch op.Op {
	case model.OpAdd:
		if idx >= 0 {
			return domainerr.AlreadyExists("upstream pool " + string(id) + " already exists")
		}
		var p model.UpstreamPool
		if err := decodeValue(op.Value, &p, path+".value"); err != nil {
			return err
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			return idMismatch(path, string(id), string(p.ID))
		}
		st.Spec.Forwarding.Pools = append(st.Spec.Forwarding.Pools, p)
	case model.OpUpdate:
		if idx < 0 {
			return domainerr.NotFound("upstream pool " + string(id) + " not found")
		}
		var p model.UpstreamPool
		if err := decodeValue(op.Value, &p, path+".value"); err != nil {
			return err
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			return idMismatch(path, string(id), string(p.ID))
		}
		st.Spec.Forwarding.Pools[idx] = p
	case model.OpRemove:
		if idx < 0 {
			return domainerr.NotFound("upstream pool " + string(id) + " not found")
		}
		st.Spec.Forwarding.Pools = append(st.Spec.Forwarding.Pools[:idx], st.Spec.Forwarding.Pools[idx+1:]...)
	}
	return nil
}

// upstreamValue carries poolId for add. Target has no parent-pool field.
type upstreamValue struct {
	ID        model.UpstreamID `json:"id"`
	Endpoint  string           `json:"endpoint"`
	Transport model.Transport  `json:"transport"`
	PoolID    model.PoolID     `json:"poolId,omitempty"`
}

func applyUpstream(st *model.State, op model.Operation, path string) error {
	id := model.UpstreamID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	pi, ui := findUpstream(st, id)
	switch op.Op {
	case model.OpAdd:
		if pi >= 0 {
			return domainerr.AlreadyExists("upstream " + string(id) + " already exists")
		}
		var v upstreamValue
		if err := decodeValue(op.Value, &v, path+".value"); err != nil {
			return err
		}
		if v.ID == "" {
			v.ID = id
		}
		if v.ID != id {
			return idMismatch(path, string(id), string(v.ID))
		}
		if v.PoolID == "" {
			return domainerr.ValidationFailed("poolId is required to add an upstream",
				domainerr.FieldViolation{Path: path + ".value.poolId", Code: "required", Message: "poolId is required"})
		}
		pidx := -1
		for i := range st.Spec.Forwarding.Pools {
			if st.Spec.Forwarding.Pools[i].ID == v.PoolID {
				pidx = i
				break
			}
		}
		if pidx < 0 {
			return domainerr.NotFound("upstream pool " + string(v.PoolID) + " not found")
		}
		st.Spec.Forwarding.Pools[pidx].Upstreams = append(st.Spec.Forwarding.Pools[pidx].Upstreams, model.Upstream{
			ID:        v.ID,
			Endpoint:  v.Endpoint,
			Transport: v.Transport,
		})
	case model.OpUpdate:
		if pi < 0 {
			return domainerr.NotFound("upstream " + string(id) + " not found")
		}
		var v upstreamValue
		if err := decodeValue(op.Value, &v, path+".value"); err != nil {
			return err
		}
		if v.ID == "" {
			v.ID = id
		}
		if v.ID != id {
			return idMismatch(path, string(id), string(v.ID))
		}
		st.Spec.Forwarding.Pools[pi].Upstreams[ui] = model.Upstream{
			ID:        v.ID,
			Endpoint:  v.Endpoint,
			Transport: v.Transport,
		}
	case model.OpRemove:
		if pi < 0 {
			return domainerr.NotFound("upstream " + string(id) + " not found")
		}
		ups := st.Spec.Forwarding.Pools[pi].Upstreams
		st.Spec.Forwarding.Pools[pi].Upstreams = append(ups[:ui], ups[ui+1:]...)
	}
	return nil
}

func findUpstream(st *model.State, id model.UpstreamID) (poolIdx, upIdx int) {
	for i := range st.Spec.Forwarding.Pools {
		for j := range st.Spec.Forwarding.Pools[i].Upstreams {
			if st.Spec.Forwarding.Pools[i].Upstreams[j].ID == id {
				return i, j
			}
		}
	}
	return -1, -1
}

func applyClientGroup(st *model.State, op model.Operation, path string) error {
	id := model.ClientGroupID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	idx := -1
	for i := range st.Spec.Access.ClientGroups {
		if st.Spec.Access.ClientGroups[i].ID == id {
			idx = i
			break
		}
	}
	switch op.Op {
	case model.OpAdd:
		if idx >= 0 {
			return domainerr.AlreadyExists("client group " + string(id) + " already exists")
		}
		var g model.ClientGroup
		if err := decodeValue(op.Value, &g, path+".value"); err != nil {
			return err
		}
		if g.ID == "" {
			g.ID = id
		}
		if g.ID != id {
			return idMismatch(path, string(id), string(g.ID))
		}
		st.Spec.Access.ClientGroups = append(st.Spec.Access.ClientGroups, g)
	case model.OpUpdate:
		if idx < 0 {
			return domainerr.NotFound("client group " + string(id) + " not found")
		}
		var g model.ClientGroup
		if err := decodeValue(op.Value, &g, path+".value"); err != nil {
			return err
		}
		if g.ID == "" {
			g.ID = id
		}
		if g.ID != id {
			return idMismatch(path, string(id), string(g.ID))
		}
		st.Spec.Access.ClientGroups[idx] = g
	case model.OpRemove:
		if idx < 0 {
			return domainerr.NotFound("client group " + string(id) + " not found")
		}
		st.Spec.Access.ClientGroups = append(st.Spec.Access.ClientGroups[:idx], st.Spec.Access.ClientGroups[idx+1:]...)
	}
	return nil
}

func applyChaosPolicy(st *model.State, op model.Operation, path string) error {
	id := model.PolicyID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	idx := -1
	for i := range st.Spec.Chaos.Policies {
		if st.Spec.Chaos.Policies[i].ID == id {
			idx = i
			break
		}
	}
	switch op.Op {
	case model.OpAdd:
		if idx >= 0 {
			return domainerr.AlreadyExists("chaos policy " + string(id) + " already exists")
		}
		var p model.ChaosPolicy
		if err := decodeValue(op.Value, &p, path+".value"); err != nil {
			return err
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			return idMismatch(path, string(id), string(p.ID))
		}
		st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies, p)
	case model.OpUpdate:
		if idx < 0 {
			return domainerr.NotFound("chaos policy " + string(id) + " not found")
		}
		var p model.ChaosPolicy
		if err := decodeValue(op.Value, &p, path+".value"); err != nil {
			return err
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			return idMismatch(path, string(id), string(p.ID))
		}
		st.Spec.Chaos.Policies[idx] = p
	case model.OpRemove:
		if idx < 0 {
			return domainerr.NotFound("chaos policy " + string(id) + " not found")
		}
		st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies[:idx], st.Spec.Chaos.Policies[idx+1:]...)
	}
	return nil
}

func applyChaosActivation(st *model.State, op model.Operation, path string) error {
	if op.Op != model.OpUpdate {
		return domainerr.ValidationFailed("chaosActivation is update-only",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "target allows only update"})
	}
	id := model.PolicyID(op.Target.ID)
	if id == "" {
		return requiredID(path)
	}
	idx := -1
	for i := range st.Spec.Chaos.Policies {
		if st.Spec.Chaos.Policies[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return domainerr.NotFound("chaos policy " + string(id) + " not found")
	}
	var act model.ChaosActivation
	if err := decodeValue(op.Value, &act, path+".value"); err != nil {
		return err
	}
	st.Spec.Chaos.Policies[idx].Enabled = act.Enabled
	st.Spec.Chaos.Policies[idx].ExpiresAt = act.ExpiresAt
	return nil
}

func requiredID(path string) error {
	return domainerr.ValidationFailed("target id is required",
		domainerr.FieldViolation{Path: path + ".target.id", Code: "required", Message: "id is required"})
}

func idMismatch(path, want, got string) error {
	return domainerr.ValidationFailed("value id does not match target",
		domainerr.FieldViolation{Path: path + ".value.id", Code: "invalid_value",
			Message: fmt.Sprintf("value id %q does not match target id %q", got, want)})
}

var opDurationFields = map[string]bool{
	"ttl": true, "negativeTTL": true, "refresh": true, "retry": true,
	"expire": true, "minimum": true, "timeout": true, "minimumTTL": true,
	"maximumTTL": true, "maximumNegativeTTL": true, "maxDelay": true,
	"defaultMaxLifetime": true, "timeBucket": true, "period": true,
	"unhealthy": true, "phaseOffset": true, "duration": true,
	"min": true, "max": true, "hold": true,
}

func decodeValue(raw json.RawMessage, dest any, path string) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return domainerr.ValidationFailed("missing value",
			domainerr.FieldViolation{Path: path, Code: "required", Message: "value is required"})
	}
	var tree any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&tree); err != nil {
		return domainerr.ValidationFailed("invalid value",
			domainerr.FieldViolation{Path: path, Code: "invalid_value", Message: err.Error()})
	}
	if err := coerceOpDurations(tree); err != nil {
		return domainerr.ValidationFailed("invalid duration",
			domainerr.FieldViolation{Path: path, Code: "invalid_value", Message: err.Error()})
	}
	materializeOpDefaults(tree)
	b, err := json.Marshal(tree)
	if err != nil {
		return domainerr.Internal("re-encode value: " + err.Error())
	}
	out := json.NewDecoder(bytes.NewReader(b))
	out.DisallowUnknownFields()
	if err := out.Decode(dest); err != nil {
		return domainerr.ValidationFailed("invalid value",
			domainerr.FieldViolation{Path: path, Code: "invalid_value", Message: err.Error()})
	}
	return nil
}

func coerceOpDurations(v any) error {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if opDurationFields[k] {
				switch n := child.(type) {
				case string:
					d, err := time.ParseDuration(n)
					if err != nil {
						return fmt.Errorf("%s: duration must use Go time.ParseDuration syntax", k)
					}
					x[k] = int64(d)
				case json.Number, float64, int64, int:
					// encoding/json time.Duration is nanoseconds; tests marshal that way
				case nil:
				default:
					return fmt.Errorf("%s: duration must be a string such as 30s or a JSON number", k)
				}
				continue
			}
			if err := coerceOpDurations(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := coerceOpDurations(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func materializeOpDefaults(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	// Go bool zero cannot distinguish omitted allowForward from explicit false.
	if _, exists := m["allowForward"]; !exists {
		if _, hasCIDRs := m["cidrs"]; hasCIDRs {
			m["allowForward"] = true
		}
	}
	if groups, ok := m["clientGroups"].([]any); ok {
		for _, g := range groups {
			materializeOpDefaults(g)
		}
	}
}

func injectUIEnabled(raw json.RawMessage) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return raw
	}
	if _, exists := m["enabled"]; exists {
		return raw
	}
	m["enabled"] = true
	b, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return b
}
