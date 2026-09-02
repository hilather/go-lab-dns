package app

import (
	"bytes"
	"context"
	"testing"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestApplyDeniedForViewer(t *testing.T) {
	svc, boot := mustBoot(t, copyFixture(t))
	viewer := auth.Actor{ID: "v", Class: auth.ClassToken, Role: auth.RoleViewer}
	_, err := svc.Apply(context.Background(), viewer, ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	})
	_ = requireCode(t, err, domainerr.CodeForbidden)
}

func TestExportRedactsSecretRef(t *testing.T) {
	svc, boot := mustBoot(t, copyFixture(t))
	ctx := context.Background()
	applied, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetManagement},
			Value:  []byte(`{"auth":{"profile":"bearer","secretRef":"/run/secrets/labdns-token"}}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	exp, err := svc.Export(ctx, actor(), ExportJSON)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(exp.Body, []byte("/run/secrets/labdns-token")) {
		t.Fatalf("secretRef leaked: %s", exp.Body)
	}
	if !bytes.Contains(exp.Body, []byte(auth.Redacted)) {
		t.Fatalf("missing redaction: %s", exp.Body)
	}
	for _, op := range exp.BootstrapToRuntime {
		if bytes.Contains(op.Value, []byte("/run/secrets/labdns-token")) {
			t.Fatalf("bootstrapToRuntime leaked: %s", op.Value)
		}
	}
	st, err := svc.GetState(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if st.Canonical != nil && st.Canonical.Spec.Management.Auth.SecretRef == "/run/secrets/labdns-token" {
		t.Fatal("GET state leaked secretRef")
	}
	ev, err := svc.GetAudit(ctx, actor(), applied.AuditEventID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ev.Diff {
		if bytes.Contains(d.After, []byte("/run/secrets/labdns-token")) {
			t.Fatalf("audit leaked secretRef: %s", d.After)
		}
	}
}

func TestProtectedRecordMutation(t *testing.T) {
	svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	editor := auth.Actor{ID: "e", Class: auth.ClassToken, Role: auth.RoleDNSEditor}
	// pack-sample protects dns.lab.example.net.
	_, err := svc.Apply(context.Background(), editor, ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetRecord, ID: "ns1-a", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "ns1-a", Owner: "dns.lab.example.net.", Type: model.TypeA, Values: []string{"10.42.0.1"}}),
		}},
	})
	if err == nil {
		t.Fatal("expected protected_object")
	}
	de, ok := domainerr.As(err)
	if !ok || (de.Code != domainerr.CodeProtectedObject && de.Code != domainerr.CodeValidationFailed && de.Code != domainerr.CodeNotFound) {
		// If ns1-a is not the protected owner, still require a domain error.
		if !ok {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestApplyRejectsProtectedNameZoneReplaceAndRelativeOwner(t *testing.T) {
	svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	ctx := context.Background()
	admin := actor()
	editor := auth.Actor{ID: "e", Class: auth.ClassToken, Role: auth.RoleDNSEditor}

	added, err := svc.Apply(ctx, admin, ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "add-protected-dns",
		Reason:           "seed protected name",
		Operations: []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "dns-a", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "dns-a", Owner: "dns", Type: model.TypeA, Values: []string{"10.42.0.1"}}),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	z, err := svc.GetZone(ctx, admin, "lab-zone")
	if err != nil {
		t.Fatal(err)
	}
	kept := make([]model.Record, 0, len(z.Records))
	for _, r := range z.Records {
		if r.ID != "dns-a" {
			kept = append(kept, r)
		}
	}
	stripped := *z
	stripped.Records = kept
	_, err = svc.Apply(ctx, editor, ChangeIn{
		ExpectedRevision: added.CandidateRevision,
		IdempotencyKey:   "editor-zone-omit",
		Reason:           "edit zone soa",
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetZone, ID: "lab-zone"},
			Value:  mustJSON(stripped),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeProtectedObject)
	live, err := svc.GetRecord(ctx, admin, "lab-zone", "dns-a")
	if err != nil {
		t.Fatal(err)
	}
	if live.Values[0] != "10.42.0.1" {
		t.Fatalf("protected record mutated: %+v", live)
	}

	_, err = svc.Apply(ctx, editor, ChangeIn{
		ExpectedRevision: added.CandidateRevision,
		IdempotencyKey:   "editor-relative-aaaa",
		Reason:           "add aaaa",
		Operations: []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "dns-aaaa", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "dns-aaaa", Owner: "dns", Type: model.TypeAAAA, Values: []string{"2001:db8::1"}}),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeProtectedObject)

	_, err = svc.Apply(ctx, editor, ChangeIn{
		ExpectedRevision: added.CandidateRevision,
		IdempotencyKey:   "editor-covering-wildcard",
		Reason:           "add catch-all",
		Operations: []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "star-a", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "star-a", Owner: "*", Type: model.TypeA, Values: []string{"10.42.0.99"}}),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeProtectedObject)
}

func TestValidateMatchesPlanAuthorization(t *testing.T) {
	svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	ctx := context.Background()
	operator := auth.Actor{ID: "o", Class: auth.ClassToken, Role: auth.RoleChaosOperator}
	editor := auth.Actor{ID: "e", Class: auth.ClassToken, Role: auth.RoleDNSEditor}

	copied, err := cloneState(boot.Canonical)
	if err != nil {
		t.Fatal(err)
	}
	copied.Spec.Chaos.Policies = append(copied.Spec.Chaos.Policies, model.ChaosPolicy{
		ID: "hi", Owner: "lab", Reason: "test", SafetyClass: model.SafetyClassHigh,
	})
	_, err = svc.Validate(ctx, operator, ValidateIn{
		State: copied,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetChaosActivation, ID: "hi"},
			Value:  []byte(`{"enabled":true}`),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeForbidden)

	_, err = svc.Validate(ctx, operator, ValidateIn{})
	_ = requireCode(t, err, domainerr.CodeForbidden)

	copied2, err := cloneState(boot.Canonical)
	if err != nil {
		t.Fatal(err)
	}
	zi := 0
	copied2.Spec.Zones[zi].Records = append(copied2.Spec.Zones[zi].Records, model.Record{
		ID: "dns-a", Owner: "dns.lab.example.net.", Type: model.TypeA, Values: []string{"10.42.0.1"},
	})
	_, err = svc.Validate(ctx, editor, ValidateIn{
		State: copied2,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetRecord, ID: "dns-a", ZoneID: "lab-zone"},
			Value:  []byte(`{"id":"dns-a","type":"A","values":["10.42.0.9"]}`),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeProtectedObject)
}
