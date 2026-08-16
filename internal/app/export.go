package app

import (
	"bytes"
	"context"
	"sort"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

const deploymentGuidance = "Copy the canonical document into the environment dns.yaml in the deployment repository and open a reviewable pull request. LabDNS does not write the bootstrap mount."

// Export returns canonical YAML or JSON of the active snapshot plus
// bootstrap-to-runtime operations. Comments are not preserved.
func (s *App) Export(ctx context.Context, actor Actor, format ExportFormat) (*Export, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	if format == "" {
		format = ExportYAML
	}
	if format != ExportYAML && format != ExportJSON {
		return nil, domainerr.ValidationFailed("unknown export format",
			domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"})
	}
	var body []byte
	switch format {
	case ExportJSON:
		body, err = config.CanonicalJSON(snap.Canonical)
	default:
		body, err = config.CanonicalYAML(snap.Canonical)
	}
	if err != nil {
		return nil, asDomain(err)
	}
	var boot *model.State
	if b := s.store.Bootstrap(); b != nil {
		boot = b.Canonical
	}
	ops, err := bootstrapToRuntime(boot, snap.Canonical)
	if err != nil {
		return nil, err
	}
	_, human, err := diffStates(boot, snap.Canonical)
	if err != nil {
		return nil, err
	}
	return &Export{
		Format:             format,
		Body:               auth.RedactBytes(append([]byte(nil), body...)),
		Revision:           snap.Revision,
		BootstrapRevision:  snap.BootstrapRevision,
		Drifted:            drifted(snap),
		BootstrapToRuntime: ops,
		HumanDiff:          auth.RedactString(human),
		DeploymentGuidance: deploymentGuidance,
	}, nil
}

func bootstrapToRuntime(boot, runtime *model.State) ([]model.Operation, error) {
	if boot == nil {
		boot = &model.State{}
	}
	if runtime == nil {
		runtime = &model.State{}
	}
	var ops []model.Operation
	ops = append(ops, singletonOps(model.TargetListeners, boot.Spec.Listeners, runtime.Spec.Listeners)...)
	ops = append(ops, singletonOps(model.TargetDefaults, boot.Spec.Defaults, runtime.Spec.Defaults)...)
	ops = append(ops, singletonOps(model.TargetCache, boot.Spec.Cache, runtime.Spec.Cache)...)
	ops = append(ops, singletonOps(model.TargetObservability, boot.Spec.Observability, runtime.Spec.Observability)...)
	ops = append(ops, singletonOps(model.TargetManagement, boot.Spec.Management, runtime.Spec.Management)...)
	ops = append(ops, singletonOps(model.TargetChaosSafety, boot.Spec.Chaos.Safety, runtime.Spec.Chaos.Safety)...)
	if !bytes.Equal(mustJSON(boot.Spec.Access), mustJSON(runtime.Spec.Access)) {
		// Replace the whole access object so group add/remove stay apply-able
		// without a second pass over clientGroups.
		ops = append(ops, model.Operation{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetAccess},
			Value:  mustJSON(runtime.Spec.Access),
		})
	}

	bz := zonesByID(boot)
	rz := zonesByID(runtime)
	ids := sortedZoneIDs(bz, rz)
	for _, id := range ids {
		b, bOk := bz[id]
		r, rOk := rz[id]
		switch {
		case !bOk:
			ops = append(ops, model.Operation{
				Op:     model.OpAdd,
				Target: model.Target{Kind: model.TargetZone, ID: string(id)},
				Value:  mustJSON(r),
			})
		case !rOk:
			ops = append(ops, model.Operation{
				Op:     model.OpRemove,
				Target: model.Target{Kind: model.TargetZone, ID: string(id)},
			})
		case zoneMetaChanged(b, r):
			ops = append(ops, model.Operation{
				Op:     model.OpUpdate,
				Target: model.Target{Kind: model.TargetZone, ID: string(id)},
				Value:  mustJSON(r),
			})
		default:
			ops = append(ops, recordOps(id, b, r)...)
		}
	}

	ops = append(ops, keyedOps(model.TargetForwardingPolicy, policiesByID(boot.Spec.Forwarding.Policies), policiesByID(runtime.Spec.Forwarding.Policies))...)
	ops = append(ops, keyedOps(model.TargetUpstreamPool, poolsByID(boot.Spec.Forwarding.Pools), poolsByID(runtime.Spec.Forwarding.Pools))...)
	ops = append(ops, keyedOps(model.TargetChaosPolicy, chaosByID(boot.Spec.Chaos.Policies), chaosByID(runtime.Spec.Chaos.Policies))...)
	return ops, nil
}

func singletonOps(kind model.TargetKind, before, after any) []model.Operation {
	if bytes.Equal(mustJSON(before), mustJSON(after)) {
		return nil
	}
	return []model.Operation{{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: kind},
		Value:  mustJSON(after),
	}}
}

func keyedOps[T any](kind model.TargetKind, before, after map[string]T) []model.Operation {
	keys := map[string]struct{}{}
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}
	ids := make([]string, 0, len(keys))
	for k := range keys {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	var ops []model.Operation
	for _, id := range ids {
		b, bOk := before[id]
		a, aOk := after[id]
		switch {
		case !bOk:
			ops = append(ops, model.Operation{
				Op:     model.OpAdd,
				Target: model.Target{Kind: kind, ID: id},
				Value:  mustJSON(a),
			})
		case !aOk:
			ops = append(ops, model.Operation{
				Op:     model.OpRemove,
				Target: model.Target{Kind: kind, ID: id},
			})
		case !bytes.Equal(mustJSON(b), mustJSON(a)):
			ops = append(ops, model.Operation{
				Op:     model.OpUpdate,
				Target: model.Target{Kind: kind, ID: id},
				Value:  mustJSON(a),
			})
		}
	}
	return ops
}

func recordOps(zone model.ZoneID, before, after model.Zone) []model.Operation {
	br := recordsByID(before)
	ar := recordsByID(after)
	keys := map[model.RecordID]struct{}{}
	for k := range br {
		keys[k] = struct{}{}
	}
	for k := range ar {
		keys[k] = struct{}{}
	}
	ids := make([]string, 0, len(keys))
	for k := range keys {
		ids = append(ids, string(k))
	}
	sort.Strings(ids)
	var ops []model.Operation
	for _, id := range ids {
		rid := model.RecordID(id)
		b, bOk := br[rid]
		a, aOk := ar[rid]
		switch {
		case !bOk:
			ops = append(ops, model.Operation{
				Op:     model.OpAdd,
				Target: model.Target{Kind: model.TargetRecord, ID: id, ZoneID: zone},
				Value:  mustJSON(a),
			})
		case !aOk:
			ops = append(ops, model.Operation{
				Op:     model.OpRemove,
				Target: model.Target{Kind: model.TargetRecord, ID: id, ZoneID: zone},
			})
		case !recordEqual(b, a):
			ops = append(ops, model.Operation{
				Op:     model.OpUpdate,
				Target: model.Target{Kind: model.TargetRecord, ID: id, ZoneID: zone},
				Value:  mustJSON(a),
			})
		}
	}
	return ops
}

func zoneMetaChanged(a, b model.Zone) bool {
	a.Records = nil
	b.Records = nil
	return !bytes.Equal(mustJSON(a), mustJSON(b))
}

func sortedZoneIDs(a, b map[model.ZoneID]model.Zone) []model.ZoneID {
	set := map[model.ZoneID]struct{}{}
	for id := range a {
		set[id] = struct{}{}
	}
	for id := range b {
		set[id] = struct{}{}
	}
	ids := make([]model.ZoneID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func policiesByID(in []model.ForwardingPolicy) map[string]model.ForwardingPolicy {
	out := map[string]model.ForwardingPolicy{}
	for _, p := range in {
		out[string(p.ID)] = p
	}
	return out
}

func poolsByID(in []model.UpstreamPool) map[string]model.UpstreamPool {
	out := map[string]model.UpstreamPool{}
	for _, p := range in {
		out[string(p.ID)] = p
	}
	return out
}

func chaosByID(in []model.ChaosPolicy) map[string]model.ChaosPolicy {
	out := map[string]model.ChaosPolicy{}
	for _, p := range in {
		out[string(p.ID)] = p
	}
	return out
}
