package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

const (
	authZoneID    model.ZoneID = "lab-zone"
	overlayZoneID model.ZoneID = "vendor-overlay"
	origin                     = model.Name("lab.example.net.")
	overlayOrigin              = model.Name("vendor.example.")
)

func mustCompile(t *testing.T, zones []model.Zone) snapshot.ZoneIndex {
	t.Helper()
	idx, err := Compile(&model.State{Spec: model.Spec{Zones: zones}})
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func snapOf(t *testing.T, zones []model.Zone, depth int) *snapshot.Snapshot {
	t.Helper()
	idx := mustCompile(t, zones)
	if depth == 0 {
		depth = model.DefaultCNAMEDepth
	}
	return &snapshot.Snapshot{
		Zones: idx,
		Defaults: snapshot.DefaultsView{
			TTL:         30 * time.Second,
			NegativeTTL: 10 * time.Second,
			CNAMEDepth:  depth,
		},
		Revision:   "sha256:test",
		Generation: 7,
	}
}

func authZone(records ...model.Record) model.Zone {
	return model.Zone{
		ID:   authZoneID,
		Name: origin,
		Mode: model.ZoneModeAuthoritative,
		SOA: &model.SOA{
			Primary:       "ns1.lab.example.net.",
			Administrator: "hostmaster.lab.example.net.",
			Serial:        "auto",
			Refresh:       time.Hour,
			Retry:         5 * time.Minute,
			Expire:        24 * time.Hour,
			Minimum:       10 * time.Second,
		},
		Nameservers: []model.Name{"ns1.lab.example.net."},
		Records:     records,
	}
}

func overlayZone(records ...model.Record) model.Zone {
	return model.Zone{
		ID:      overlayZoneID,
		Name:    overlayOrigin,
		Mode:    model.ZoneModeOverlay,
		Records: records,
	}
}

func rec(id, owner string, typ model.RRType, ttl time.Duration, values ...string) model.Record {
	return model.Record{
		ID:     model.RecordID(id),
		Owner:  owner,
		Type:   typ,
		TTL:    ttl,
		Values: values,
	}
}

func resolve(t *testing.T, snap *snapshot.Snapshot, name string, typ model.RRType, zone model.ZoneID) model.Result {
	t.Helper()
	res, err := Resolve(context.Background(), snap, model.Query{
		Name:  model.Name(name),
		Type:  typ,
		Class: model.ClassIN,
	}, zone)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func wantRCode(t *testing.T, res model.Result, want model.RCode) {
	t.Helper()
	if res.RCode != want {
		t.Fatalf("rcode=%s, want %s (source=%s fallthrough=%v answers=%d)", res.RCode, want, res.Source, res.Fallthrough, len(res.Answers))
	}
}

func wantData(t *testing.T, res model.Result, typ model.RRType, data string) {
	t.Helper()
	for _, rr := range res.Answers {
		if rr.Type == typ && rr.Data == data {
			return
		}
	}
	t.Fatalf("missing %s %q in %+v", typ, data, res.Answers)
}

func wantOwner(t *testing.T, res model.Result, typ model.RRType, owner model.Name) {
	t.Helper()
	for _, rr := range res.Answers {
		if rr.Type == typ && rr.Name == owner {
			return
		}
	}
	t.Fatalf("missing owner %s %s in %+v", owner, typ, res.Answers)
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
