package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/model"
)

func diffStates(before, after *model.State) ([]DiffEntry, string, error) {
	bt, err := jsonTree(before)
	if err != nil {
		return nil, "", err
	}
	at, err := jsonTree(after)
	if err != nil {
		return nil, "", err
	}
	var out []DiffEntry
	walkDiff(bt, at, "", &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, formatHumanDiff(out), nil
}

func jsonTree(st *model.State) (any, error) {
	if st == nil {
		return map[string]any{}, nil
	}
	raw, err := config.CanonicalJSON(st)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func walkDiff(before, after any, path string, out *[]DiffEntry) {
	if deepEqualJSON(before, after) {
		return
	}
	bm, bMap := before.(map[string]any)
	am, aMap := after.(map[string]any)
	if bMap && aMap {
		keys := map[string]struct{}{}
		for k := range bm {
			keys[k] = struct{}{}
		}
		for k := range am {
			keys[k] = struct{}{}
		}
		ks := make([]string, 0, len(keys))
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			bv, bOk := bm[k]
			av, aOk := am[k]
			p := joinJSONPath(path, k)
			switch {
			case !bOk:
				*out = append(*out, DiffEntry{Path: p, Op: "add", After: mustJSON(av)})
			case !aOk:
				*out = append(*out, DiffEntry{Path: p, Op: "remove", Before: mustJSON(bv)})
			default:
				walkDiff(bv, av, p, out)
			}
		}
		return
	}
	bs, bArr := before.([]any)
	as, aArr := after.([]any)
	if bArr && aArr {
		n := len(bs)
		if len(as) > n {
			n = len(as)
		}
		for i := 0; i < n; i++ {
			p := path + "[" + strconv.Itoa(i) + "]"
			switch {
			case i >= len(bs):
				*out = append(*out, DiffEntry{Path: p, Op: "add", After: mustJSON(as[i])})
			case i >= len(as):
				*out = append(*out, DiffEntry{Path: p, Op: "remove", Before: mustJSON(bs[i])})
			default:
				walkDiff(bs[i], as[i], p, out)
			}
		}
		return
	}
	op := "update"
	if before == nil {
		op = "add"
	} else if after == nil {
		op = "remove"
	}
	*out = append(*out, DiffEntry{Path: pathOrRoot(path), Op: op, Before: mustJSON(before), After: mustJSON(after)})
}

func joinJSONPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func pathOrRoot(p string) string {
	if p == "" {
		return "$"
	}
	return p
}

func deepEqualJSON(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

func formatHumanDiff(diff []DiffEntry) string {
	if len(diff) == 0 {
		return ""
	}
	var b strings.Builder
	for _, d := range diff {
		fmt.Fprintf(&b, "%s %s\n", d.Op, d.Path)
		if len(d.Before) > 0 && d.Op != "add" {
			fmt.Fprintf(&b, "  - %s\n", string(d.Before))
		}
		if len(d.After) > 0 && d.Op != "remove" {
			fmt.Fprintf(&b, "  + %s\n", string(d.After))
		}
	}
	return b.String()
}

func impactOf(before, after *model.State, diff []DiffEntry) Impact {
	imp := Impact{
		RequiredPermissions: []string{"dns.write"},
	}
	zoneSet := map[model.ZoneID]struct{}{}
	nameSet := map[model.Name]struct{}{}
	groupSet := map[model.ClientGroupID]struct{}{}
	if before == nil {
		before = &model.State{}
	}
	if after == nil {
		after = &model.State{}
	}

	beforeZones := zonesByID(before)
	afterZones := zonesByID(after)
	for id, az := range afterZones {
		bz, ok := beforeZones[id]
		if !ok {
			zoneSet[id] = struct{}{}
			imp.AuthoritativeMisses = true
			collectRecordNames(az, nameSet, &imp.WildcardCoverage)
			continue
		}
		if bz.Mode != az.Mode || bz.Name != az.Name {
			zoneSet[id] = struct{}{}
			imp.AuthoritativeMisses = true
		}
		br := recordsByID(bz)
		ar := recordsByID(az)
		for rid, rec := range ar {
			old, exists := br[rid]
			if !exists || !recordEqual(old, rec) {
				zoneSet[id] = struct{}{}
				collectOneName(az, rec, nameSet, &imp.WildcardCoverage)
			}
			// New or removed owners change NXDOMAIN/NODATA vs NOERROR.
			if !exists || ownerKey(old) != ownerKey(rec) {
				imp.AuthoritativeMisses = true
			}
		}
		for rid := range br {
			if _, ok := ar[rid]; !ok {
				zoneSet[id] = struct{}{}
				collectOneName(bz, br[rid], nameSet, &imp.WildcardCoverage)
				imp.AuthoritativeMisses = true
			}
		}
	}
	for id, bz := range beforeZones {
		if _, ok := afterZones[id]; !ok {
			zoneSet[id] = struct{}{}
			imp.AuthoritativeMisses = true
			collectRecordNames(bz, nameSet, &imp.WildcardCoverage)
		}
	}

	if !deepEqualJSON(before.Spec.Forwarding, after.Spec.Forwarding) {
		imp.ForwardingChanged = true
	}
	if !deepEqualJSON(before.Spec.Access.ClientGroups, after.Spec.Access.ClientGroups) {
		for _, g := range after.Spec.Access.ClientGroups {
			groupSet[g.ID] = struct{}{}
		}
		for _, g := range before.Spec.Access.ClientGroups {
			groupSet[g.ID] = struct{}{}
		}
	}

	imp.ChaosPolicies = chaosImpact(before, after)

	for _, d := range diff {
		if strings.Contains(d.Path, "forwarding") {
			imp.ForwardingChanged = true
		}
		if strings.Contains(d.Path, "clientGroups") || strings.Contains(d.Path, "access") {
			imp.RequiredPermissions = uniqStrings(append(imp.RequiredPermissions, "dns.write"))
		}
	}

	for id := range zoneSet {
		imp.Zones = append(imp.Zones, id)
	}
	sort.Slice(imp.Zones, func(i, j int) bool { return imp.Zones[i] < imp.Zones[j] })
	for n := range nameSet {
		imp.Names = append(imp.Names, n)
	}
	sort.Slice(imp.Names, func(i, j int) bool { return imp.Names[i] < imp.Names[j] })
	for g := range groupSet {
		imp.ClientGroups = append(imp.ClientGroups, g)
	}
	sort.Slice(imp.ClientGroups, func(i, j int) bool { return imp.ClientGroups[i] < imp.ClientGroups[j] })

	for _, n := range imp.Names {
		imp.SuggestedProbes = append(imp.SuggestedProbes, "resolve "+string(n)+" A")
	}
	return imp
}

func zonesByID(st *model.State) map[model.ZoneID]model.Zone {
	out := map[model.ZoneID]model.Zone{}
	if st == nil {
		return out
	}
	for _, z := range st.Spec.Zones {
		out[z.ID] = z
	}
	return out
}

func recordsByID(z model.Zone) map[model.RecordID]model.Record {
	out := map[model.RecordID]model.Record{}
	for _, r := range z.Records {
		out[r.ID] = r
	}
	return out
}

func recordEqual(a, b model.Record) bool {
	return bytes.Equal(mustJSON(a), mustJSON(b))
}

func ownerKey(r model.Record) string {
	return strings.ToLower(strings.TrimSpace(r.Owner))
}

func collectRecordNames(z model.Zone, names map[model.Name]struct{}, wild *bool) {
	for _, r := range z.Records {
		collectOneName(z, r, names, wild)
	}
}

func collectOneName(z model.Zone, r model.Record, names map[model.Name]struct{}, wild *bool) {
	owner := strings.ToLower(strings.TrimSpace(r.Owner))
	if strings.HasPrefix(owner, "*.") || owner == "*" {
		*wild = true
	}
	if owner == "" {
		return
	}
	if strings.HasSuffix(owner, ".") {
		names[model.Name(owner)] = struct{}{}
		return
	}
	origin := strings.TrimSuffix(string(z.Name), ".")
	if origin == "" {
		names[model.Name(owner+".")] = struct{}{}
		return
	}
	names[model.Name(owner+"."+origin+".")] = struct{}{}
}

func chaosImpact(before, after *model.State) []ChaosImpact {
	type key struct {
		enabled bool
		exp     string
	}
	prev := map[model.PolicyID]key{}
	for _, p := range before.Spec.Chaos.Policies {
		prev[p.ID] = key{enabled: p.Enabled, exp: expKey(p.ExpiresAt)}
	}
	var out []ChaosImpact
	seen := map[model.PolicyID]struct{}{}
	for _, p := range after.Spec.Chaos.Policies {
		seen[p.ID] = struct{}{}
		old, ok := prev[p.ID]
		if !ok || old.enabled != p.Enabled || old.exp != expKey(p.ExpiresAt) {
			out = append(out, ChaosImpact{ID: p.ID, Enabled: p.Enabled, ExpiresAt: cloneTime(p.ExpiresAt)})
		}
	}
	for _, p := range before.Spec.Chaos.Policies {
		if _, ok := seen[p.ID]; !ok {
			out = append(out, ChaosImpact{ID: p.ID, Enabled: false, ExpiresAt: cloneTime(p.ExpiresAt)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func expKey(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func uniqStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
