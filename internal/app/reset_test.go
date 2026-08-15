package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestResetRestoresBootstrap(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	applied, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "before-reset",
		Operations:       []model.Operation{addWWWRecord()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Drifted {
		t.Fatal("expected drift after apply")
	}

	res, err := svc.Reset(ctx, actor(), ResetIn{Reason: "restore"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("reset did not apply")
	}
	live := svc.Store().Load()
	if live.Revision != boot.Revision {
		t.Fatalf("reset rev=%s boot=%s", live.Revision, boot.Revision)
	}
	if live.Revision != live.BootstrapRevision {
		t.Fatal("reset should clear drift")
	}
	if res.Drifted {
		t.Fatal("reset result still drifted")
	}
	for _, r := range live.Canonical.Spec.Zones[0].Records {
		if r.ID == "www-a" {
			t.Fatal("runtime record survived reset")
		}
	}

	// Idempotency cache must be cleared so the pre-reset key can be reused
	// with a new body against the restored revision.
	_, err = svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: live.Revision,
		IdempotencyKey:   "before-reset",
		Reason:           "after reset",
		Operations:       []model.Operation{addWWWRecord()},
	})
	if err != nil {
		t.Fatalf("reset should have cleared idempotency: %v", err)
	}
}

func TestFailedResetKeepsIdempotencyCache(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "keep-me",
		Reason:           "first",
		Operations:       []model.Operation{addWWWRecord()},
	}
	if _, err := svc.Apply(ctx, actor(), in); err != nil {
		t.Fatal(err)
	}
	svc.bootstrapPath = filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := svc.Reset(ctx, actor(), ResetIn{}); err == nil {
		t.Fatal("expected failed reset")
	}
	in.Reason = "second"
	in.ExpectedRevision = svc.Store().Load().Revision
	_, err := svc.Apply(ctx, actor(), in)
	requireCode(t, err, domainerr.CodeIdempotencyConflict)
}

func TestResetMissingFileLeavesActive(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	live := svc.Store().Load()
	svc.bootstrapPath = filepath.Join(t.TempDir(), "missing.yaml")
	_, err := svc.Reset(ctx, actor(), ResetIn{})
	requireCode(t, err, domainerr.CodeValidationFailed)
	if svc.Store().Load() != live {
		t.Fatal("missing bootstrap reset swapped active")
	}
}

func TestResetInvalidFileLeavesActive(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	live := svc.Store().Load()
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("not: valid: labdns\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.bootstrapPath = bad
	_, err := svc.Reset(ctx, actor(), ResetIn{})
	if err == nil {
		t.Fatal("invalid bootstrap reset succeeded")
	}
	if svc.Store().Load() != live {
		t.Fatal("invalid bootstrap reset swapped active")
	}
}

func TestResetDoesNotWriteBootstrapFile(t *testing.T) {
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
	if _, err := svc.Reset(context.Background(), actor(), ResetIn{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("reset wrote the bootstrap file")
	}
}

func TestResetWithoutPathUsesLastBootstrapState(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	svc.bootstrapPath = ""
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Reset(ctx, actor(), ResetIn{})
	if err != nil {
		t.Fatal(err)
	}
	if res.CandidateRevision != boot.Revision {
		t.Fatalf("reset rev=%s boot=%s", res.CandidateRevision, boot.Revision)
	}
}
