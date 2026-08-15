package compiler

import (
	"context"
	"fmt"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/resolver"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// CompileOpts controls revision metadata and the compile clock.
type CompileOpts struct {
	Clock             testutil.Clock
	BootstrapRevision model.Revision
	Generation        model.Generation
	EmergencyChaosOff bool
}

// Compile normalizes and validates st (copy-on-write), fills domain indexes,
// hashes canonical JSON for Revision, and returns an immutable Snapshot.
//
// Always normalize+validate rather than guessing whether the caller already
// did; Normalize does not mutate st.
func Compile(ctx context.Context, st *model.State, opts CompileOpts) (*snapshot.Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n, err := config.Normalize(st)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(n); err != nil {
		return nil, err
	}

	zones, err := resolver.Compile(n)
	if err != nil {
		return nil, fmt.Errorf("compile zones: %w", err)
	}
	fwd, err := forwarder.Compile(n)
	if err != nil {
		return nil, fmt.Errorf("compile forwarding: %w", err)
	}
	access, err := snapshot.CompileAccess(n)
	if err != nil {
		return nil, fmt.Errorf("compile access: %w", err)
	}
	ch, err := chaos.Compile(n)
	if err != nil {
		return nil, fmt.Errorf("compile chaos: %w", err)
	}

	rev, err := config.Revision(n)
	if err != nil {
		return nil, err
	}
	bootRev := opts.BootstrapRevision
	if bootRev == "" {
		bootRev = rev
	}

	clk := opts.Clock
	if clk == nil {
		clk = testutil.SystemClock{}
	}

	return &snapshot.Snapshot{
		Canonical:         n,
		Revision:          rev,
		BootstrapRevision: bootRev,
		Generation:        opts.Generation,
		CompiledAt:        clk.Now(),
		Listeners:         listenerView(n),
		Access:            access,
		Defaults: snapshot.DefaultsView{
			TTL:         n.Spec.Defaults.TTL,
			NegativeTTL: n.Spec.Defaults.NegativeTTL,
			CNAMEDepth:  n.Spec.Defaults.CNAMEDepth,
		},
		Zones:      zones,
		Forwarding: fwd,
		Chaos:      ch,
		CachePolicy: snapshot.CachePolicy{
			Enabled:            n.Spec.Cache.Enabled,
			MaxEntries:         n.Spec.Cache.MaxEntries,
			MinimumTTL:         n.Spec.Cache.MinimumTTL,
			MaximumTTL:         n.Spec.Cache.MaximumTTL,
			MaximumNegativeTTL: n.Spec.Cache.MaximumNegativeTTL,
			StaleServing:       n.Spec.Cache.StaleServing,
		},
		Safety:            safetyView(n),
		Management:        snapshot.ManagementView{AuthProfile: n.Spec.Management.Auth.Profile},
		Observability:     snapshot.ObservabilityView{LogQNAME: n.Spec.Observability.LogQNAME},
		EmergencyChaosOff: opts.EmergencyChaosOff || n.Spec.Chaos.EmergencyDisabled,
	}, nil
}

func listenerView(st *model.State) snapshot.ListenerView {
	return snapshot.ListenerView{
		DNSAddress:        st.Spec.Listeners.DNS.Address,
		DNSProtocols:      append([]model.Transport(nil), st.Spec.Listeners.DNS.Protocols...),
		ManagementAddress: st.Spec.Listeners.Management.Address,
		RESTPath:          st.Spec.Listeners.Management.RESTPath,
		MCPPath:           st.Spec.Listeners.Management.MCPPath,
	}
}

func safetyView(st *model.State) snapshot.SafetyPolicy {
	s := st.Spec.Chaos.Safety
	return snapshot.SafetyPolicy{
		ProtectedNames:                append([]model.Name(nil), s.ProtectedNames...),
		ProtectedClientGroups:         append([]model.ClientGroupID(nil), s.ProtectedClientGroups...),
		AllowedAddressCIDRs:           append([]string(nil), s.AllowedAddressCIDRs...),
		MaxDelay:                      s.MaxDelay,
		MaxConcurrentDelayed:          s.MaxConcurrentDelayed,
		MaxDropProbability:            s.MaxDropProbability,
		MaxActiveHighImpactPolicies:   s.MaxActiveHighImpactPolicies,
		RequireExpiryForSafetyClasses: append([]model.SafetyClass(nil), s.RequireExpiryForSafetyClasses...),
		DefaultMaxLifetime:            s.DefaultMaxLifetime,
	}
}
