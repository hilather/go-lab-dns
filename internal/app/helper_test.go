package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func fixturePath(t *testing.T) string {
	t.Helper()
	return namedFixture(t, "empty-client-groups.yaml")
}

func namedFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "config", "valid", name)
}

func copyFixture(t *testing.T) string {
	t.Helper()
	return copyNamedFixture(t, "empty-client-groups.yaml")
}

func copyNamedFixture(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(namedFixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, src, 0o444); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustBoot(t *testing.T, path string) (*App, *snapshot.Snapshot) {
	t.Helper()
	st, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	svc := New(Options{Store: store, Clock: clk, BootstrapPath: path})
	return svc, snap
}

func addWWWRecord() model.Operation {
	return model.Operation{
		Op:     model.OpAdd,
		Target: model.Target{Kind: model.TargetRecord, ID: "www-a", ZoneID: "lab-zone"},
		Value:  mustJSON(model.Record{ID: "www-a", Owner: "www", Type: model.TypeA, Values: []string{"10.42.0.80"}}),
	}
}

func requireCode(t *testing.T, err error, code domainerr.Code) *domainerr.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	de, ok := domainerr.As(err)
	if !ok {
		t.Fatalf("err=%T %v, want domain %s", err, err, code)
	}
	if de.Code != code {
		t.Fatalf("code=%s want %s (%v)", de.Code, code, err)
	}
	return de
}

func actor() Actor {
	return Actor{ID: "test", Class: "token", Role: "administrator"}
}
