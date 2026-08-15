package compiler

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestCompileNilState(t *testing.T) {
	_, err := Compile(context.Background(), nil, CompileOpts{})
	if err == nil {
		t.Fatal("nil state compiled")
	}
	if !errors.Is(err, domainerr.New(domainerr.CodeValidationFailed, "")) {
		var de *domainerr.Error
		if !errors.As(err, &de) || de.Code != domainerr.CodeValidationFailed {
			t.Fatalf("err=%v, want validation_failed", err)
		}
	}
}

func TestCompileCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Compile(ctx, &model.State{}, CompileOpts{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}

func TestCompilePackSampleAccessAndRevision(t *testing.T) {
	st := loadPack(t)
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	snap, err := Compile(context.Background(), st, CompileOpts{Clock: clk, Generation: 0})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Canonical == st {
		t.Fatal("Compile must not retain the caller pointer")
	}
	wantRev, err := config.Revision(st)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Revision != wantRev || snap.BootstrapRevision != wantRev {
		t.Fatalf("revision=%s bootstrap=%s want %s", snap.Revision, snap.BootstrapRevision, wantRev)
	}
	if snap.CompiledAt != clk.Now() {
		t.Fatalf("CompiledAt=%s", snap.CompiledAt)
	}
	if !snap.Access.Compiled() {
		t.Fatal("AccessIndex not compiled")
	}
	if id, allow := snap.Access.Classify(netip.MustParseAddr("10.42.1.1")); id != "test-devices" || !allow {
		t.Fatalf("test-devices: id=%s allow=%v", id, allow)
	}
	if id, allow := snap.Access.Classify(netip.MustParseAddr("10.42.255.9")); id != "management" || !allow {
		t.Fatalf("management: id=%s allow=%v", id, allow)
	}
	if id, allow := snap.Access.Classify(netip.MustParseAddr("127.0.0.1")); id != "" || allow {
		t.Fatalf("loopback must be unknown: id=%s allow=%v", id, allow)
	}
	if _, ok := snap.Zones.Select("ns1.lab.example.net."); !ok {
		t.Fatal("lab zone missing")
	}
	if snap.Listeners.DNSAddress != ":5353" {
		t.Fatalf("dns listen %q", snap.Listeners.DNSAddress)
	}
	if snap.EmergencyChaosOff {
		t.Fatal("pack-sample must not compile with emergency off")
	}
	if !snap.Chaos.Compiled() || !snap.Chaos.Enabled {
		t.Fatal("pack-sample chaos index missing")
	}
	if _, ok := snap.Chaos.Lookup("slow-tools"); !ok {
		t.Fatal("slow-tools not indexed")
	}
}

func TestCompileDeterministicForSameCanonicalJSON(t *testing.T) {
	st := loadPack(t)
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	a, err := Compile(context.Background(), st, CompileOpts{Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	// Reload so the second compile cannot share in-memory maps with the first.
	st2 := loadPack(t)
	b, err := Compile(context.Background(), st2, CompileOpts{Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	if a.Revision != b.Revision {
		t.Fatalf("revision drifted\n%s\n%s", a.Revision, b.Revision)
	}
	ja, err := config.CanonicalJSON(a.Canonical)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := config.CanonicalJSON(b.Canonical)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ja, jb) {
		t.Fatal("canonical JSON differed for the same pack-sample")
	}
	idA, allowA := a.Access.Classify(netip.MustParseAddr("10.42.255.9"))
	idB, allowB := b.Access.Classify(netip.MustParseAddr("10.42.255.9"))
	if idA != idB || allowA != allowB {
		t.Fatalf("access classify drifted %s/%v vs %s/%v", idA, allowA, idB, allowB)
	}
}

func TestCompileDoesNotMutateInput(t *testing.T) {
	st := loadPack(t)
	before := st.Spec.Access.ClientGroups[0].ID
	snap, err := Compile(context.Background(), st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.Access.ClientGroups[0].ID = "mutated"
	if snap.Canonical.Spec.Access.ClientGroups[0].ID != before {
		t.Fatal("Compile mutated caller state into Canonical")
	}
	id, _ := snap.Access.Classify(netip.MustParseAddr("10.42.1.1"))
	if id != before {
		t.Fatalf("AccessIndex followed mutated spec id=%s", id)
	}
}

func TestCompileInvalidRejected(t *testing.T) {
	st := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "bad"},
		Spec: model.Spec{
			Access: model.AccessSpec{
				UnknownClient: model.UnknownClientRefuseForward,
				ClientGroups:  []model.ClientGroup{{ID: "g", CIDRs: []string{"not-a-cidr"}}},
			},
		},
	}
	snap, err := Compile(context.Background(), st, CompileOpts{})
	if err == nil || snap != nil {
		t.Fatal("invalid CIDR compiled")
	}
}

func TestCompileBootstrapRevisionOverride(t *testing.T) {
	st := loadPack(t)
	snap, err := Compile(context.Background(), st, CompileOpts{
		BootstrapRevision: "sha256:deadbeef",
		Generation:        3,
		EmergencyChaosOff: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.BootstrapRevision != "sha256:deadbeef" {
		t.Fatalf("bootstrap=%s", snap.BootstrapRevision)
	}
	if snap.Revision == snap.BootstrapRevision {
		t.Fatal("runtime revision must stay content-hashed")
	}
	if snap.Generation != 3 || !snap.EmergencyChaosOff {
		t.Fatalf("opts not applied gen=%d off=%v", snap.Generation, snap.EmergencyChaosOff)
	}
}

func loadPack(t *testing.T) *model.State {
	t.Helper()
	st, err := config.LoadFile(filepath.Join(repoRoot(t), "testdata/config/valid/pack-sample.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

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
