package snapshot

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestInZoneLabelBoundary(t *testing.T) {
	if InZone("notlab.example.net.", "lab.example.net.") {
		t.Fatal("suffix without label boundary")
	}
	if !InZone("foo.lab.example.net.", "lab.example.net.") {
		t.Fatal("child should be in zone")
	}
	if !InZone("lab.example.net.", "lab.example.net.") {
		t.Fatal("apex should be in zone")
	}
	if !InZone("anything.example.", ".") {
		t.Fatal("root contains all names")
	}
	if InZone("lab.example.net.", "") {
		t.Fatal("empty zone contains nothing")
	}
}

func TestParentAndWildcardHelpers(t *testing.T) {
	if got := ParentName("foo.lab.example.net."); got != "lab.example.net." {
		t.Fatalf("parent=%q", got)
	}
	if got := ParentName("lab."); got != "." {
		t.Fatalf("parent lab.=%q", got)
	}
	if got := ParentName("."); got != "" {
		t.Fatalf("parent .=%q", got)
	}
	if !IsWildcardOwner("*.tools.lab.") || !IsWildcardOwner("*.") {
		t.Fatal("wildcard owner")
	}
	if IsWildcardOwner("x*.tools.lab.") || IsWildcardOwner("star.tools.lab.") {
		t.Fatal("non-wildcard treated as wildcard")
	}
	if got := WildcardOwner("tools.lab.example.net."); got != "*.tools.lab.example.net." {
		t.Fatalf("wc=%q", got)
	}
	if got := WildcardOwner("."); got != "*." {
		t.Fatalf("root wc=%q", got)
	}
}

func TestSelectLongestSuffix(t *testing.T) {
	idx := ZoneIndex{ByID: map[model.ZoneID]*ZoneData{
		"lab":    {ID: "lab", Name: "lab.example.net."},
		"tools":  {ID: "tools", Name: "tools.lab.example.net."},
		"root":   {ID: "root", Name: "."},
		"vendor": {ID: "vendor", Name: "vendor.example."},
	}}
	got, ok := idx.Select("a.tools.lab.example.net.")
	if !ok || got != "tools" {
		t.Fatalf("got=%s ok=%v", got, ok)
	}
	got, ok = idx.Select("ns1.lab.example.net.")
	if !ok || got != "lab" {
		t.Fatalf("got=%s ok=%v", got, ok)
	}
	got, ok = idx.Select("other.")
	if !ok || got != "root" {
		t.Fatalf("root fallback got=%s ok=%v", got, ok)
	}
}
