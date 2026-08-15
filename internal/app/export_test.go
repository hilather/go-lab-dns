package app

import (
	"bytes"
	"context"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestExportDeterministic(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	a, err := svc.Export(ctx, actor(), ExportYAML)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Export(ctx, actor(), ExportYAML)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Body, b.Body) {
		t.Fatal("YAML export was not deterministic")
	}
	ja, err := svc.Export(ctx, actor(), ExportJSON)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := svc.Export(ctx, actor(), ExportJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ja.Body, jb.Body) {
		t.Fatal("JSON export was not deterministic")
	}
	if !a.Drifted || !ja.Drifted {
		t.Fatal("export should report drift after apply")
	}
	if bytes.Contains(a.Body, []byte("#")) {
		t.Fatal("canonical YAML must not include comments")
	}
}

func TestExportBootstrapToRuntimeOps(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	ex, err := svc.Export(ctx, actor(), ExportJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.BootstrapToRuntime) == 0 {
		t.Fatal("expected bootstrap-to-runtime operations")
	}
	found := false
	for _, op := range ex.BootstrapToRuntime {
		if op.Op == model.OpAdd && op.Target.Kind == model.TargetRecord && op.Target.ID == "www-a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ops=%+v", ex.BootstrapToRuntime)
	}
	if ex.HumanDiff == "" {
		t.Fatal("expected human-readable diff")
	}
	if ex.DeploymentGuidance == "" {
		t.Fatal("expected deployment guidance")
	}

	// Applying the exported ops to a fresh bootstrap must land on the same revision.
	fresh, _ := mustBoot(t, copyFixture(t))
	applied, err := fresh.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: fresh.Store().Load().Revision,
		Operations:       ex.BootstrapToRuntime,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied.CandidateRevision != ex.Revision {
		t.Fatalf("replay rev=%s export=%s", applied.CandidateRevision, ex.Revision)
	}
}

func TestExportUnknownFormat(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	_, err := svc.Export(context.Background(), actor(), ExportFormat("xml"))
	requireCode(t, err, "validation_failed")
}
