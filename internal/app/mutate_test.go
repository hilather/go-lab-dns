package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestPlanDoesNotSwapApplyDoes(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		Reason:           "add www",
		Operations:       []model.Operation{addWWWRecord()},
	}

	plan, err := svc.Plan(ctx, actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PreviousRevision != boot.Revision {
		t.Fatalf("plan prev=%s", plan.PreviousRevision)
	}
	if plan.CandidateRevision == boot.Revision {
		t.Fatal("plan candidate should change")
	}
	if svc.Store().Load() != boot {
		t.Fatal("plan swapped the live snapshot")
	}
	if svc.Store().Load().Revision != boot.Revision {
		t.Fatal("plan changed revision")
	}

	applied, err := svc.Apply(ctx, actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatal("apply did not apply")
	}
	live := svc.Store().Load()
	if live == boot {
		t.Fatal("apply kept the bootstrap pointer")
	}
	if live.Revision != plan.CandidateRevision {
		t.Fatalf("apply rev=%s plan=%s", live.Revision, plan.CandidateRevision)
	}
	if live.Generation != boot.Generation+1 {
		t.Fatalf("generation=%d", live.Generation)
	}
	if !applied.Drifted {
		t.Fatal("apply should report drift")
	}
	if !plan.Impact.AuthoritativeMisses {
		t.Fatal("adding an owner must flag authoritative-miss changes")
	}
	found := false
	for _, r := range live.Canonical.Spec.Zones[0].Records {
		if r.ID == "www-a" {
			found = true
		}
	}
	if !found {
		t.Fatal("applied record missing from canonical")
	}
}

func TestRevisionConflict(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: "sha256:deadbeef",
		Operations:       []model.Operation{addWWWRecord()},
	})
	de := requireCode(t, err, domainerr.CodeRevisionConflict)
	if de.CurrentRevision != string(boot.Revision) {
		t.Fatalf("currentRevision=%s want %s", de.CurrentRevision, boot.Revision)
	}
	if svc.Store().Load() != boot {
		t.Fatal("conflict mutated active")
	}
}

func TestMissingExpectedRevisionFailClosed(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	_, err := svc.Plan(context.Background(), actor(), ChangeIn{
		Operations: []model.Operation{addWWWRecord()},
	})
	_ = requireCode(t, err, domainerr.CodeValidationFailed)
	if svc.Store().Load() != boot {
		t.Fatal("missing revision mutated active")
	}
}

func TestIdempotencySameKeySameBody(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "k1",
		Reason:           "once",
		Operations:       []model.Operation{addWWWRecord()},
	}
	a, err := svc.Apply(context.Background(), actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Apply(context.Background(), actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if a.CandidateRevision != b.CandidateRevision || a.AuditEventID != b.AuditEventID {
		t.Fatalf("idempotent replay differed %+v vs %+v", a, b)
	}
	if svc.Store().Load().Generation != boot.Generation+1 {
		t.Fatal("replay applied a second generation")
	}
}

func TestIdempotencyPlanThenApplySameKey(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "shared",
		Reason:           "promote",
		Operations:       []model.Operation{addWWWRecord()},
	}
	plan, err := svc.Plan(context.Background(), actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := svc.Apply(context.Background(), actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if applied.CandidateRevision != plan.CandidateRevision {
		t.Fatalf("apply rev=%s plan=%s", applied.CandidateRevision, plan.CandidateRevision)
	}
	again, err := svc.Apply(context.Background(), actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if again.AuditEventID != applied.AuditEventID {
		t.Fatal("apply replay was not cached")
	}
}

func TestIdempotencyRetryAfterRevisionConflict(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	const key = "shared-after-conflict"
	planIn := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   key,
		Reason:           "add www",
		Operations:       []model.Operation{addWWWRecord()},
	}
	if _, err := svc.Plan(ctx, actor(), planIn); err != nil {
		t.Fatal(err)
	}
	// Foreign apply moves the revision using a different key.
	foreign, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "foreign",
		Reason:           "other",
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetDefaults},
			Value:  json.RawMessage(`{"ttl":"45s","negativeTTL":"10s","cnameDepth":8}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(ctx, actor(), planIn)
	_ = requireCode(t, err, domainerr.CodeRevisionConflict)
	planIn.ExpectedRevision = foreign.CandidateRevision
	if _, err := svc.Apply(ctx, actor(), planIn); err != nil {
		t.Fatalf("retry after revision_conflict should succeed: %v", err)
	}
	found := false
	for _, r := range svc.Store().Load().Canonical.Spec.Zones[0].Records {
		if r.ID == "www-a" {
			found = true
		}
	}
	if !found {
		t.Fatal("retry apply did not land the record")
	}
}

func TestIdempotencySameKeyDifferentBody(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "k1",
		Reason:           "once",
		Operations:       []model.Operation{addWWWRecord()},
	}
	if _, err := svc.Apply(context.Background(), actor(), in); err != nil {
		t.Fatal(err)
	}
	in.Reason = "different"
	_, err := svc.Apply(context.Background(), actor(), in)
	_ = requireCode(t, err, domainerr.CodeIdempotencyConflict)
}

func TestInvalidOperationLeavesActiveUnchanged(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     "splice",
			Target: model.Target{Kind: model.TargetRecord, ID: "x", ZoneID: "lab-zone"},
		}},
	})
	_ = requireCode(t, err, domainerr.CodeValidationFailed)
	if svc.Store().Load() != boot {
		t.Fatal("invalid op swapped active")
	}

	_, err = svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "x"},
			Value:  mustJSON(model.Record{ID: "x", Owner: "x", Type: model.TypeA, Values: []string{"1.2.3.4"}}),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeValidationFailed)

	_, err = svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "ns1-a", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "ns1-a", Owner: "ns1", Type: model.TypeA, Values: []string{"1.2.3.4"}}),
		}},
	})
	_ = requireCode(t, err, domainerr.CodeAlreadyExists)

	_, err = svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpRemove,
			Target: model.Target{Kind: model.TargetRecord, ID: "nope", ZoneID: "lab-zone"},
		}},
	})
	_ = requireCode(t, err, domainerr.CodeNotFound)

	if svc.Store().Load() != boot {
		t.Fatal("failed applies swapped active")
	}
}

func TestInvalidCandidateCompileLeavesActiveUnchanged(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	// CNAME next to the existing A at ns1 is rejected at validate.
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpAdd,
			Target: model.Target{Kind: model.TargetRecord, ID: "ns1-cname", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "ns1-cname", Owner: "ns1", Type: model.TypeCNAME, Values: []string{"other.lab.example.net."}}),
		}},
	})
	if err == nil {
		t.Fatal("coexisting CNAME compiled")
	}
	if !errors.Is(err, domainerr.New(domainerr.CodeValidationFailed, "")) {
		if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeValidationFailed {
			t.Fatalf("err=%v, want validation_failed", err)
		}
	}
	if svc.Store().Load() != boot {
		t.Fatal("failed compile swapped active")
	}
}

func TestApplyDoesNotWriteBootstrapFile(t *testing.T) {
	path := copyFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	svc, boot := mustBoot(t, path)
	if _, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("apply wrote the bootstrap file")
	}
}

func TestCopyOnWriteReturnedState(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	st, err := svc.GetState(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	st.Canonical.Spec.Zones[0].Records[0].Values[0] = "9.9.9.9"
	live := svc.Store().Load()
	if live.Canonical.Spec.Zones[0].Records[0].Values[0] == "9.9.9.9" {
		t.Fatal("caller mutated live canonical")
	}
}

func TestNoActiveSnapshotFailClosed(t *testing.T) {
	svc := New(Options{})
	_, err := svc.Plan(context.Background(), actor(), ChangeIn{
		ExpectedRevision: "sha256:x",
		Operations:       []model.Operation{addWWWRecord()},
	})
	_ = requireCode(t, err, domainerr.CodeInternalError)
	_, err = svc.GetState(context.Background(), actor())
	_ = requireCode(t, err, domainerr.CodeInternalError)
}

func TestPreviousSnapshotIsSingleGeneration(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	first, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: first.CandidateRevision,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab-zone"},
			Value:  mustJSON(model.Record{ID: "www-a", Owner: "www", Type: model.TypeA, Values: []string{"10.42.0.81"}}),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prev := svc.Store().Previous()
	if prev == nil || prev.Revision != first.CandidateRevision {
		t.Fatalf("previous=%v want first apply", prev)
	}
	if svc.Store().Bootstrap() != boot {
		t.Fatal("bootstrap pointer replaced by apply")
	}
	_ = second
}

func TestValidateDoesNotRequireRevision(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	p, err := svc.Validate(context.Background(), actor(), ValidateIn{
		Operations: []model.Operation{addWWWRecord()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.CandidateRevision == p.PreviousRevision {
		t.Fatal("validate should compile a new candidate")
	}
}

func TestPlanCanceledContext(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.Plan(ctx, actor(), ChangeIn{ExpectedRevision: boot.Revision})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestClonePlanCopiesOperationValue(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "raw",
		Operations:       []model.Operation{addWWWRecord()},
	}
	plan, err := svc.Plan(context.Background(), actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) == 0 || len(plan.Operations[0].Value) == 0 {
		t.Fatal("plan missing op value")
	}
	plan.Operations[0].Value[0] = 'X'
	in.Operations[0].Value[0] = 'Y'
	again, err := svc.Plan(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "raw",
		Operations:       []model.Operation{addWWWRecord()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.Operations[0].Value[0] == 'X' || again.Operations[0].Value[0] == 'Y' {
		t.Fatal("cached plan shared RawMessage")
	}
}

func TestDurationStringValue(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	raw := json.RawMessage(`{"ttl":"45s","negativeTTL":"5s","cnameDepth":8}`)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetDefaults},
			Value:  raw,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Store().Load().Canonical.Spec.Defaults.TTL.String() != "45s" {
		t.Fatalf("ttl=%s", svc.Store().Load().Canonical.Spec.Defaults.TTL)
	}
}
