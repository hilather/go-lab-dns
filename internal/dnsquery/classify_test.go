package dnsquery

import (
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestClassifyLongestPrefixAndUnknown(t *testing.T) {
	snap := &snapshot.Snapshot{
		Canonical: &model.State{Spec: model.Spec{Access: model.AccessSpec{
			ClientGroups: []model.ClientGroup{
				{ID: "wide", CIDRs: []string{"10.42.0.0/16"}, AllowForward: true},
				{ID: "mgmt", CIDRs: []string{"10.42.255.0/24"}, AllowForward: false},
			},
		}}},
	}
	id, allow := classifyClient(snap, netip.MustParseAddr("10.42.255.9"))
	if id != "mgmt" || allow {
		t.Fatalf("longest prefix: id=%s allow=%v", id, allow)
	}
	id, allow = classifyClient(snap, netip.MustParseAddr("10.42.1.1"))
	if id != "wide" || !allow {
		t.Fatalf("wide: id=%s allow=%v", id, allow)
	}
	id, allow = classifyClient(snap, netip.MustParseAddr("127.0.0.1"))
	if id != "" || allow {
		t.Fatalf("unknown: id=%s allow=%v", id, allow)
	}
}

func TestClassifyCompiledIndexWinsOverSpec(t *testing.T) {
	empty, err := snapshot.CompileAccess(&model.State{})
	if err != nil {
		t.Fatal(err)
	}
	snap := &snapshot.Snapshot{
		Access: empty,
		Canonical: &model.State{Spec: model.Spec{Access: model.AccessSpec{
			ClientGroups: []model.ClientGroup{
				{ID: "spec-only", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true},
			},
		}}},
	}
	id, allow := classifyClient(snap, netip.MustParseAddr("10.1.1.1"))
	if id != "" || allow {
		t.Fatalf("compiled-empty must not fall back to spec: id=%s allow=%v", id, allow)
	}
}

func TestClassifyNoForwardWithoutPolicy(t *testing.T) {
	snap := &snapshot.Snapshot{
		Canonical: &model.State{Spec: model.Spec{Access: model.AccessSpec{
			ClientGroups: []model.ClientGroup{{ID: "g", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true}},
		}}},
	}
	cl := classify(snap, model.Query{Name: "x.example.", Client: netip.MustParseAddr("10.1.1.1")})
	if cl.Group != "g" || !cl.AllowForward || cl.ForwardingID != "" {
		t.Fatalf("%+v", cl)
	}
}
