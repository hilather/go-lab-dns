package resolver

import (
	"errors"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestCompileNilAndEmpty(t *testing.T) {
	idx, err := Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.Select("lab.example.net."); ok {
		t.Fatal("empty index selected a zone")
	}
	idx, err = Compile(&model.State{})
	if err != nil {
		t.Fatal(err)
	}
	if idx.ByID == nil {
		t.Fatal("Compile empty state left ByID nil")
	}
}

func TestCompileRejectsWildcardNS(t *testing.T) {
	_, err := Compile(&model.State{Spec: model.Spec{Zones: []model.Zone{
		overlayZone(rec("wns", "*", model.TypeNS, time.Second, "ns1.vendor.example.")),
	}}})
	if !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileRejectsWildcardDNAME(t *testing.T) {
	_, err := Compile(&model.State{Spec: model.Spec{Zones: []model.Zone{
		overlayZone(model.Record{
			ID:    "wd",
			Owner: "*",
			Type:  "DNAME",
			TTL:   time.Second,
			GenericRDATA: &model.GenericRDATA{
				TypeCode:     39,
				Presentation: "other.example.",
			},
		}),
	}}})
	if !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileRejectsCNAMECoexist(t *testing.T) {
	_, err := Compile(&model.State{Spec: model.Spec{Zones: []model.Zone{
		overlayZone(
			rec("c", "x", model.TypeCNAME, time.Second, "y.vendor.example."),
			rec("a", "x", model.TypeA, time.Second, "192.0.2.1"),
		),
	}}})
	if !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileRejectsCNAMELoop(t *testing.T) {
	_, err := Compile(&model.State{Spec: model.Spec{Zones: []model.Zone{
		overlayZone(
			rec("a", "a", model.TypeCNAME, time.Second, "b.vendor.example."),
			rec("b", "b", model.TypeCNAME, time.Second, "a.vendor.example."),
		),
	}}})
	if !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileRejectsAuthApexCNAME(t *testing.T) {
	_, err := Compile(&model.State{Spec: model.Spec{Zones: []model.Zone{
		authZone(rec("c", "lab.example.net.", model.TypeCNAME, time.Second, "x.lab.example.net.")),
	}}})
	if !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileEmptyNonTerminals(t *testing.T) {
	idx := mustCompile(t, []model.Zone{
		authZone(rec("deep", "a.b.tools", model.TypeA, time.Second, "192.0.2.9")),
	})
	zd, ok := idx.Lookup(authZoneID)
	if !ok {
		t.Fatal("missing zone")
	}
	for _, name := range []model.Name{
		"lab.example.net.",
		"tools.lab.example.net.",
		"b.tools.lab.example.net.",
		"a.b.tools.lab.example.net.",
	} {
		if !zd.HasName(name) {
			t.Fatalf("expected existence of %s", name)
		}
	}
	if zd.HasName("missing.lab.example.net.") {
		t.Fatal("missing name exists")
	}
}

func TestCompileCopiesRRsetData(t *testing.T) {
	values := []string{"192.0.2.1"}
	st := &model.State{Spec: model.Spec{Zones: []model.Zone{
		authZone(rec("a", "ns1", model.TypeA, time.Second, values...)),
	}}}
	idx, err := Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	values[0] = "192.0.2.99"
	st.Spec.Zones[0].Records[0].Values[0] = "192.0.2.77"
	zd, _ := idx.Lookup(authZoneID)
	rr, ok := zd.RRset("ns1.lab.example.net.", model.TypeA)
	if !ok || rr.Data[0] != "192.0.2.1" {
		t.Fatalf("compile did not copy values: %+v", rr)
	}
	got := zd.AllRRsets("ns1.lab.example.net.")
	got[0].Data[0] = "mutated"
	rr2, _ := zd.RRset("ns1.lab.example.net.", model.TypeA)
	if rr2.Data[0] != "192.0.2.1" {
		t.Fatal("RRset helper leaked backing array")
	}
}

func TestCompileRelativeOwners(t *testing.T) {
	idx := mustCompile(t, []model.Zone{
		authZone(rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53")),
	})
	zd, _ := idx.Lookup(authZoneID)
	if _, ok := zd.RRset("ns1.lab.example.net.", model.TypeA); !ok {
		t.Fatal("relative owner not expanded")
	}
}

func TestSelectMostSpecific(t *testing.T) {
	idx := mustCompile(t, []model.Zone{
		authZone(rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53")),
		{
			ID:   "tools-overlay",
			Name: "tools.lab.example.net.",
			Mode: model.ZoneModeOverlay,
			Records: []model.Record{
				rec("w", "*", model.TypeA, time.Second, "10.42.0.20"),
			},
		},
	})
	id, ok := idx.Select("alpha.tools.lab.example.net.")
	if !ok || id != "tools-overlay" {
		t.Fatalf("select=%s ok=%v", id, ok)
	}
	id, ok = idx.Select("ns1.lab.example.net.")
	if !ok || id != authZoneID {
		t.Fatalf("select ns1=%s ok=%v", id, ok)
	}
	if _, ok := idx.Select("other.example."); ok {
		t.Fatal("selected unmatched name")
	}
}

func TestCompileApexNSUsesDefaultsTTL(t *testing.T) {
	withTTL, err := Compile(&model.State{Spec: model.Spec{
		Defaults: model.DefaultsSpec{TTL: 30 * time.Second},
		Zones:    []model.Zone{authZone()},
	}})
	if err != nil {
		t.Fatal(err)
	}
	zd, _ := withTTL.Lookup(authZoneID)
	rr, ok := zd.RRset(origin, model.TypeNS)
	if !ok || rr.TTL != 30*time.Second {
		t.Fatalf("apex NS TTL=%v ok=%v, want 30s from defaults", rr.TTL, ok)
	}

	noTTL, err := Compile(&model.State{Spec: model.Spec{Zones: []model.Zone{authZone()}}})
	if err != nil {
		t.Fatal(err)
	}
	zd, _ = noTTL.Lookup(authZoneID)
	rr, ok = zd.RRset(origin, model.TypeNS)
	if !ok || rr.TTL != 0 {
		t.Fatalf("unpopulated defaults must leave apex NS TTL 0, got %v ok=%v", rr.TTL, ok)
	}
}

func TestZeroValueZoneIndex(t *testing.T) {
	var idx snapshot.ZoneIndex
	if _, ok := idx.Select("lab.example.net."); ok {
		t.Fatal("zero Select hit")
	}
	if _, ok := idx.Lookup("x"); ok {
		t.Fatal("zero Lookup hit")
	}
	var zd *snapshot.ZoneData
	if zd.HasName("x.") || zd.Contains("x.") {
		t.Fatal("nil ZoneData should miss")
	}
}
