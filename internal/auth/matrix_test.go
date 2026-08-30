package auth

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// capRow is one registry row used by the role×capability matrix.
type capRow struct {
	ID       string
	Required []string
}

func firstGACapabilities() []capRow {
	return []capRow{
		{"health.live", nil},
		{"health.ready", nil},
		{"version", []string{ScopeDNSRead}},
		{"capabilities", []string{ScopeDNSRead}},
		{"status", []string{ScopeDNSRead}},
		{"schema.config", []string{ScopeDNSRead}},
		{"state.get", []string{ScopeDNSRead}},
		{"state.validate", []string{ScopeDNSWrite}},
		{"change.plan", []string{ScopeDNSWrite}},
		{"change.apply", []string{ScopeDNSWrite}},
		{"state.export", []string{ScopeDNSRead}},
		{"state.reset", []string{ScopeDNSAdmin}},
		{"zones", []string{ScopeDNSRead}},
		{"records", []string{ScopeDNSRead}},
		{"resolve", []string{ScopeDNSRead}},
		{"resolve.explain", []string{ScopeDNSRead}},
		{"forwarding.policies", []string{ScopeForwardersRead}},
		{"upstream.pools", []string{ScopeForwardersRead}},
		{"upstreams.status", []string{ScopeForwardersRead}},
		{"cache.status", []string{ScopeDNSRead}},
		{"cache.flush", []string{ScopeDNSAdmin}},
		{"chaos.status", []string{ScopeChaosRead}},
		{"chaos.policies", []string{ScopeChaosRead}},
		{"chaos.simulate", []string{ScopeChaosRead}},
		{"chaos.activate", []string{ScopeChaosActivate}},
		{"chaos.set_expiry", []string{ScopeChaosActivate}},
		{"chaos.emergency", []string{ScopeChaosEmergency}},
		{"audit.list", []string{ScopeAuditRead}},
		{"audit.get", []string{ScopeAuditRead}},
		{"docs.dns-semantics", []string{ScopeDNSRead}},
		{"docs.chaos-safety", []string{ScopeDNSRead}},
	}
}

func TestRoleCapabilityMatrix(t *testing.T) {
	// Expected allow set: capability ID → roles that pass AuthorizeCapability.
	// Plan/apply/validate are resource-aware (any mutate-family scope).
	allow := map[string]map[string]bool{}
	mark := func(cap string, roles ...string) {
		if allow[cap] == nil {
			allow[cap] = map[string]bool{}
		}
		for _, r := range roles {
			allow[cap][r] = true
		}
	}
	all := AllRoles()
	readers := []string{RoleViewer, RoleDNSEditor, RoleForwarderOperator, RoleChaosDesigner, RoleChaosOperator, RoleChaosAdmin, RoleEmergencyOperator, RoleAdministrator}
	fwdReaders := []string{RoleViewer, RoleForwarderOperator, RoleAdministrator}
	chaosReaders := []string{RoleViewer, RoleChaosDesigner, RoleChaosOperator, RoleChaosAdmin, RoleEmergencyOperator, RoleAdministrator}
	mutators := []string{RoleDNSEditor, RoleForwarderOperator, RoleChaosDesigner, RoleChaosOperator, RoleChaosAdmin, RoleEmergencyOperator, RoleAdministrator}

	for _, cap := range []string{"health.live", "health.ready"} {
		mark(cap, all...)
	}
	for _, cap := range []string{"version", "capabilities", "status", "schema.config", "state.get", "state.export", "zones", "records", "resolve", "resolve.explain", "cache.status", "docs.dns-semantics", "docs.chaos-safety"} {
		mark(cap, readers...)
	}
	mark("state.validate", mutators...)
	mark("change.plan", mutators...)
	mark("change.apply", mutators...)
	mark("state.reset", RoleAdministrator)
	mark("cache.flush", RoleAdministrator)
	mark("forwarding.policies", fwdReaders...)
	mark("upstream.pools", fwdReaders...)
	mark("upstreams.status", fwdReaders...)
	mark("chaos.status", chaosReaders...)
	mark("chaos.policies", chaosReaders...)
	mark("chaos.simulate", chaosReaders...)
	mark("chaos.activate", RoleChaosOperator, RoleChaosAdmin, RoleAdministrator)
	mark("chaos.set_expiry", RoleChaosOperator, RoleChaosAdmin, RoleAdministrator)
	mark("chaos.emergency", RoleChaosAdmin, RoleEmergencyOperator, RoleAdministrator)
	mark("audit.list", RoleAdministrator)
	mark("audit.get", RoleAdministrator)

	for _, row := range firstGACapabilities() {
		for _, role := range all {
			actor := Actor{ID: role, Class: ClassToken, Role: role}
			err := AuthorizeCapability(actor, row.Required, row.ID)
			want := allow[row.ID][role]
			if want && err != nil {
				t.Errorf("role %s cap %s: want allow, got %v", role, row.ID, err)
			}
			if !want && err == nil {
				t.Errorf("role %s cap %s: want deny", role, row.ID)
			}
			if !want && err != nil {
				if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeForbidden {
					t.Errorf("role %s cap %s: want forbidden, got %v", role, row.ID, err)
				}
			}
		}
	}
}

func TestChaosPrivilegeSeparation(t *testing.T) {
	low := model.ChaosPolicy{ID: "p1", Enabled: false, SafetyClass: model.SafetyClassLow}
	high := model.ChaosPolicy{ID: "p2", Enabled: false, SafetyClass: model.SafetyClassHigh}
	st := &model.State{}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{low, high}

	designer := Actor{ID: "d", Class: ClassToken, Role: RoleChaosDesigner}
	operator := Actor{ID: "o", Class: ClassToken, Role: RoleChaosOperator}
	admin := Actor{ID: "a", Class: ClassToken, Role: RoleChaosAdmin}

	createDisabled := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetChaosPolicy, ID: "new"},
		Value:  []byte(`{"id":"new","enabled":false,"safetyClass":"low"}`),
	}
	if err := AuthorizeChange(designer, []model.Operation{createDisabled}, st); err != nil {
		t.Fatalf("designer create disabled: %v", err)
	}
	createEnabled := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetChaosPolicy, ID: "live"},
		Value:  []byte(`{"id":"live","enabled":true,"safetyClass":"low"}`),
	}
	if err := AuthorizeChange(designer, []model.Operation{createEnabled}, st); err == nil {
		t.Fatal("designer must not create enabled policy")
	}
	activateLow := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosActivation, ID: "p1"},
		Value:  []byte(`{"enabled":true}`),
	}
	if err := AuthorizeChange(operator, []model.Operation{activateLow}, st); err != nil {
		t.Fatalf("operator activate low: %v", err)
	}
	if err := AuthorizeChange(designer, []model.Operation{activateLow}, st); err == nil {
		t.Fatal("designer must not activate")
	}
	activateHigh := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosActivation, ID: "p2"},
		Value:  []byte(`{"enabled":true}`),
	}
	if err := AuthorizeChange(operator, []model.Operation{activateHigh}, st); err == nil {
		t.Fatal("operator must not activate high")
	}
	if err := AuthorizeChange(admin, []model.Operation{activateHigh}, st); err != nil {
		t.Fatalf("chaos admin activate high: %v", err)
	}

	emerg := Actor{ID: "e", Class: ClassToken, Role: RoleEmergencyOperator}
	if err := AuthorizeEmergency(emerg, false); err != nil {
		t.Fatalf("emergency disable: %v", err)
	}
	if err := AuthorizeEmergency(emerg, true); err == nil {
		t.Fatal("emergency operator must not enable")
	}
	if err := AuthorizeEmergency(admin, true); err != nil {
		t.Fatalf("chaos admin enable: %v", err)
	}

	// Operator cannot rewrite policy bodies via TargetChaosPolicy (typed
	// activation is the only enable path they have).
	rewriteLow := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosPolicy, ID: "p1"},
		Value:  []byte(`{"id":"p1","enabled":true,"safetyClass":"low","outcomes":[{"id":"x"}]}`),
	}
	if err := AuthorizeChange(operator, []model.Operation{rewriteLow}, st); err == nil {
		t.Fatal("operator rewrote live policy body")
	}
	downgrade := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosPolicy, ID: "p2"},
		Value:  []byte(`{"id":"p2","enabled":true,"safetyClass":"low"}`),
	}
	if err := AuthorizeChange(operator, []model.Operation{downgrade}, st); err == nil {
		t.Fatal("operator downgraded high policy and enabled it")
	}

	// Designer cannot disable or replace an already-enabled policy.
	st.Spec.Chaos.Policies[0].Enabled = true
	disable := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosPolicy, ID: "p1"},
		Value:  []byte(`{"id":"p1","enabled":false,"safetyClass":"low"}`),
	}
	if err := AuthorizeChange(designer, []model.Operation{disable}, st); err == nil {
		t.Fatal("designer disabled live policy")
	}
	omitEnabled := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosPolicy, ID: "p1"},
		Value:  []byte(`{"id":"p1","safetyClass":"low","reason":"tweak"}`),
	}
	if err := AuthorizeChange(designer, []model.Operation{omitEnabled}, st); err == nil {
		t.Fatal("designer replaced enabled policy with omitted enabled")
	}
	if err := AuthorizeChange(admin, []model.Operation{disable}, st); err != nil {
		t.Fatalf("chaos admin disable via design write: %v", err)
	}
}

func TestProtectedObjectsOrdinaryRoles(t *testing.T) {
	st := &model.State{}
	st.Spec.Chaos.Safety.ProtectedNames = []model.Name{"dns.lab.example.net."}
	st.Spec.Chaos.Safety.ProtectedClientGroups = []model.ClientGroupID{"management"}
	st.Spec.Zones = []model.Zone{{
		ID:   "lab",
		Name: "lab.example.net.",
		Records: []model.Record{
			{ID: "dns-a", Owner: "dns.lab.example.net.", Type: "A", Values: []string{"10.0.0.1"}},
		},
	}}
	st.Spec.Forwarding.Pools = []model.UpstreamPool{{
		ID:        "p",
		Upstreams: []model.Upstream{{ID: "u1", Endpoint: "192.0.2.53:53"}},
	}}

	editor := Actor{ID: "e", Class: ClassToken, Role: RoleDNSEditor}
	fwd := Actor{ID: "f", Class: ClassToken, Role: RoleForwarderOperator}
	adm := Actor{ID: "a", Class: ClassToken, Role: RoleAdministrator}

	touch := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetRecord, ID: "dns-a", ZoneID: "lab"},
		Value:  []byte(`{"id":"dns-a","owner":"dns.lab.example.net.","type":"A","values":["10.0.0.2"]}`),
	}
	if err := AuthorizeChange(editor, []model.Operation{touch}, st); err == nil {
		t.Fatal("editor changed protected record")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeProtectedObject {
		t.Fatalf("want protected_object, got %v", err)
	}
	if err := AuthorizeChange(adm, []model.Operation{touch}, st); err != nil {
		t.Fatalf("admin protected record: %v", err)
	}

	moveUp := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetUpstream, ID: "u1"},
		Value:  []byte(`{"id":"u1","endpoint":"198.51.100.53:53","transport":"udp"}`),
	}
	if err := AuthorizeChange(fwd, []model.Operation{moveUp}, st); err == nil {
		t.Fatal("forwarder changed upstream endpoint")
	}
	if err := AuthorizeChange(adm, []model.Operation{moveUp}, st); err != nil {
		t.Fatalf("admin upstream: %v", err)
	}

	rewritePool := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetUpstreamPool, ID: "p"},
		Value:  []byte(`{"id":"p","strategy":"ordered","upstreams":[{"id":"u1","endpoint":"198.51.100.53:53","transport":"udp"}]}`),
	}
	if err := AuthorizeChange(fwd, []model.Operation{rewritePool}, st); err == nil {
		t.Fatal("forwarder rewrote pool endpoints")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeProtectedObject {
		t.Fatalf("want protected_object, got %v", err)
	}
	dropUpstream := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetUpstreamPool, ID: "p"},
		Value:  []byte(`{"id":"p","strategy":"ordered","upstreams":[]}`),
	}
	if err := AuthorizeChange(fwd, []model.Operation{dropUpstream}, st); err == nil {
		t.Fatal("forwarder dropped existing upstreams via pool replace")
	}
	removePool := model.Operation{
		Op:     model.OpRemove,
		Target: model.Target{Kind: model.TargetUpstreamPool, ID: "p"},
	}
	if err := AuthorizeChange(fwd, []model.Operation{removePool}, st); err == nil {
		t.Fatal("forwarder removed pool with existing endpoints")
	}
	addOnly := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetUpstreamPool, ID: "p"},
		Value:  []byte(`{"id":"p","strategy":"ordered","upstreams":[{"id":"u1","endpoint":"192.0.2.53:53"},{"id":"u2","endpoint":"192.0.2.54:53"}]}`),
	}
	if err := AuthorizeChange(fwd, []model.Operation{addOnly}, st); err != nil {
		t.Fatalf("forwarder adding an upstream should be allowed: %v", err)
	}
	if err := AuthorizeChange(adm, []model.Operation{rewritePool}, st); err != nil {
		t.Fatalf("admin pool rewrite: %v", err)
	}

	safety := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetChaosSafety},
		Value:  []byte(`{"maxDelay":"1s"}`),
	}
	if err := AuthorizeChange(editor, []model.Operation{safety}, st); err == nil {
		t.Fatal("editor changed safety")
	}

	// Zone replace is whole-object: omitting a protected record deletes it.
	dropProtected := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetZone, ID: "lab"},
		Value:  []byte(`{"id":"lab","name":"lab.example.net.","records":[]}`),
	}
	if err := AuthorizeChange(editor, []model.Operation{dropProtected}, st); err == nil {
		t.Fatal("editor omitted protected record via zone replace")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeProtectedObject {
		t.Fatalf("want protected_object for zone replace omit, got %v", err)
	}
	if err := AuthorizeChange(adm, []model.Operation{dropProtected}, st); err != nil {
		t.Fatalf("admin zone replace: %v", err)
	}

	removeZone := model.Operation{
		Op:     model.OpRemove,
		Target: model.Target{Kind: model.TargetZone, ID: "lab"},
	}
	if err := AuthorizeChange(editor, []model.Operation{removeZone}, st); err == nil {
		t.Fatal("editor removed zone that contains a protected name")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeProtectedObject {
		t.Fatalf("want protected_object for zone remove, got %v", err)
	}

	// Relative owners expand against the zone origin before the protected-name check.
	addRelative := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetRecord, ID: "dns-aaaa", ZoneID: "lab"},
		Value:  []byte(`{"id":"dns-aaaa","owner":"dns","type":"AAAA","values":["2001:db8::1"]}`),
	}
	if err := AuthorizeChange(editor, []model.Operation{addRelative}, st); err == nil {
		t.Fatal("editor added relative owner that expands to a protected name")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeProtectedObject {
		t.Fatalf("want protected_object for relative owner add, got %v", err)
	}

	retarget := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab"},
		Value:  []byte(`{"id":"www-a","owner":"dns","type":"A","values":["10.0.0.9"]}`),
	}
	st.Spec.Zones[0].Records = append(st.Spec.Zones[0].Records, model.Record{
		ID: "www-a", Owner: "www.lab.example.net.", Type: "A", Values: []string{"10.0.0.8"},
	})
	if err := AuthorizeChange(editor, []model.Operation{retarget}, st); err == nil {
		t.Fatal("editor retargeted a record onto a protected name via relative owner")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeProtectedObject {
		t.Fatalf("want protected_object for relative owner retarget, got %v", err)
	}
	addWWW := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetRecord, ID: "api-a", ZoneID: "lab"},
		Value:  []byte(`{"id":"api-a","owner":"api","type":"A","values":["10.0.0.10"]}`),
	}
	if err := AuthorizeChange(editor, []model.Operation{addWWW}, st); err != nil {
		t.Fatalf("editor should still add a non-protected relative owner: %v", err)
	}
}

func TestDNSEditorCannotTouchForwarders(t *testing.T) {
	editor := Actor{ID: "e", Class: ClassToken, Role: RoleDNSEditor}
	op := model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetForwardingPolicy, ID: "f1"},
		Value:  []byte(`{"id":"f1","suffix":"."}`),
	}
	if err := AuthorizeChange(editor, []model.Operation{op}, &model.State{}); err == nil {
		t.Fatal("editor wrote forwarder")
	}
}
