package snapshot

import (
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestCompileAccessNilAndEmpty(t *testing.T) {
	idx, err := CompileAccess(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !idx.Compiled() {
		t.Fatal("nil state must still produce a compiled empty index")
	}
	if id, allow := idx.Classify(netip.MustParseAddr("10.0.0.1")); id != "" || allow {
		t.Fatalf("empty index classified %s allow=%v", id, allow)
	}

	idx, err = CompileAccess(&model.State{})
	if err != nil {
		t.Fatal(err)
	}
	if !idx.Compiled() || idx.entries == nil {
		t.Fatal("empty spec must compile to a non-nil entries slice")
	}
}

func TestCompileAccessClassifiesLongestPrefix(t *testing.T) {
	idx, err := CompileAccess(&model.State{Spec: model.Spec{Access: model.AccessSpec{
		ClientGroups: []model.ClientGroup{
			{ID: "wide", CIDRs: []string{"10.42.0.0/16"}, AllowForward: true},
			{ID: "mgmt", CIDRs: []string{"10.42.255.0/24"}, AllowForward: false},
			{ID: "v6", CIDRs: []string{"2001:db8::/32"}, AllowForward: true},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	id, allow := idx.Classify(netip.MustParseAddr("10.42.255.9"))
	if id != "mgmt" || allow {
		t.Fatalf("longest prefix: id=%s allow=%v", id, allow)
	}
	id, allow = idx.Classify(netip.MustParseAddr("10.42.1.1"))
	if id != "wide" || !allow {
		t.Fatalf("wide: id=%s allow=%v", id, allow)
	}
	id, allow = idx.Classify(netip.MustParseAddr("127.0.0.1"))
	if id != "" || allow {
		t.Fatalf("unknown: id=%s allow=%v", id, allow)
	}
	id, allow = idx.Classify(netip.MustParseAddr("2001:db8::1"))
	if id != "v6" || !allow {
		t.Fatalf("v6: id=%s allow=%v", id, allow)
	}
	// IPv4-mapped IPv6 must match the IPv4 prefix, not fall through as unknown.
	id, allow = idx.Classify(netip.MustParseAddr("::ffff:10.42.1.1"))
	if id != "wide" || !allow {
		t.Fatalf("4in6: id=%s allow=%v", id, allow)
	}
	id, allow = idx.Classify(netip.Addr{})
	if id != "" || allow {
		t.Fatalf("invalid addr: id=%s allow=%v", id, allow)
	}
}

func TestCompileAccessEqualLengthKeepsFirst(t *testing.T) {
	idx, err := CompileAccess(&model.State{Spec: model.Spec{Access: model.AccessSpec{
		ClientGroups: []model.ClientGroup{
			{ID: "first", CIDRs: []string{"10.0.0.0/8"}, AllowForward: false},
			{ID: "second", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	id, allow := idx.Classify(netip.MustParseAddr("10.1.2.3"))
	if id != "first" || allow {
		t.Fatalf("tie-break: id=%s allow=%v", id, allow)
	}
}

func TestCompileAccessRejectsInvalid(t *testing.T) {
	_, err := CompileAccess(&model.State{Spec: model.Spec{Access: model.AccessSpec{
		ClientGroups: []model.ClientGroup{{ID: "", CIDRs: []string{"10.0.0.0/8"}}},
	}}})
	if err == nil {
		t.Fatal("missing id accepted")
	}
	_, err = CompileAccess(&model.State{Spec: model.Spec{Access: model.AccessSpec{
		ClientGroups: []model.ClientGroup{{ID: "g", CIDRs: nil}},
	}}})
	if err == nil {
		t.Fatal("empty CIDRs accepted")
	}
	_, err = CompileAccess(&model.State{Spec: model.Spec{Access: model.AccessSpec{
		ClientGroups: []model.ClientGroup{{ID: "g", CIDRs: []string{"not-a-cidr"}}},
	}}})
	if err == nil {
		t.Fatal("invalid CIDR accepted")
	}
}

func TestCompileAccessCopyOnWrite(t *testing.T) {
	st := &model.State{Spec: model.Spec{Access: model.AccessSpec{
		ClientGroups: []model.ClientGroup{
			{ID: "g", CIDRs: []string{"10.0.0.0/8"}, AllowForward: true},
		},
	}}}
	idx, err := CompileAccess(st)
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.Access.ClientGroups[0].CIDRs[0] = "192.0.2.0/24"
	st.Spec.Access.ClientGroups[0].ID = "mutated"
	id, allow := idx.Classify(netip.MustParseAddr("10.1.1.1"))
	if id != "g" || !allow {
		t.Fatalf("mutating spec after compile changed index: id=%s allow=%v", id, allow)
	}
	id, allow = idx.Classify(netip.MustParseAddr("192.0.2.1"))
	if id != "" || allow {
		t.Fatalf("mutated spec CIDR leaked into index: id=%s", id)
	}
}

func TestZeroAccessIndexNotCompiled(t *testing.T) {
	var idx AccessIndex
	if idx.Compiled() {
		t.Fatal("zero AccessIndex must not report compiled")
	}
	if id, allow := idx.Classify(netip.MustParseAddr("10.0.0.1")); id != "" || allow {
		t.Fatalf("zero index classified %s allow=%v", id, allow)
	}
}
