package resolver

import (
	"fmt"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestCNAMEFollowInZone(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("c", "alias", model.TypeCNAME, time.Second, "ns1.lab.example.net."),
		rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
	)}, 0)
	res := resolve(t, snap, "alias.lab.example.net.", model.TypeA, authZoneID)
	wantRCode(t, res, model.RCodeNoError)
	wantData(t, res, model.TypeCNAME, "ns1.lab.example.net.")
	wantData(t, res, model.TypeA, "10.42.0.53")
	if res.Fallthrough || !res.AA {
		t.Fatalf("%+v", res)
	}
}

func TestCNAMEQueryDoesNotFollow(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("c", "alias", model.TypeCNAME, time.Second, "ns1.lab.example.net."),
		rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
	)}, 0)
	res := resolve(t, snap, "alias.lab.example.net.", model.TypeCNAME, authZoneID)
	wantData(t, res, model.TypeCNAME, "ns1.lab.example.net.")
	for _, rr := range res.Answers {
		if rr.Type == model.TypeA {
			t.Fatal("QTYPE CNAME must not follow")
		}
	}
}

func TestWildcardCNAME(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("wc", "*.tools", model.TypeCNAME, time.Second, "ns1.lab.example.net."),
		rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
	)}, 0)
	res := resolve(t, snap, "grafana.tools.lab.example.net.", model.TypeA, authZoneID)
	wantOwner(t, res, model.TypeCNAME, "grafana.tools.lab.example.net.")
	wantData(t, res, model.TypeCNAME, "ns1.lab.example.net.")
	wantData(t, res, model.TypeA, "10.42.0.53")
	if res.Source != model.SourceWildcard {
		t.Fatalf("source=%s", res.Source)
	}
}

func TestAuthCNAMEOutsideZoneStops(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("c", "alias", model.TypeCNAME, time.Second, "outside.example."),
	)}, 0)
	res := resolve(t, snap, "alias.lab.example.net.", model.TypeA, authZoneID)
	wantRCode(t, res, model.RCodeNoError)
	wantData(t, res, model.TypeCNAME, "outside.example.")
	if res.Fallthrough {
		t.Fatal("authoritative CNAME must not fall through")
	}
	if len(res.Answers) != 1 {
		t.Fatalf("answers=%+v", res.Answers)
	}
}

func TestOverlayCNAMEToOutsideFallthrough(t *testing.T) {
	snap := snapOf(t, []model.Zone{overlayZone(
		rec("c", "alias", model.TypeCNAME, time.Second, "outside.example."),
	)}, 0)
	res := resolve(t, snap, "alias.vendor.example.", model.TypeA, overlayZoneID)
	wantRCode(t, res, model.RCodeNoError)
	wantData(t, res, model.TypeCNAME, "outside.example.")
	if !res.Fallthrough {
		t.Fatal("overlay CNAME to a forwarded name must set Fallthrough")
	}
	if res.AA {
		t.Fatal("overlay must not set AA")
	}
	if res.Source != model.SourceExact {
		t.Fatalf("source=%s", res.Source)
	}
}

func TestOverlayWildcardCNAMEToOutside(t *testing.T) {
	snap := snapOf(t, []model.Zone{overlayZone(
		rec("wc", "*", model.TypeCNAME, time.Second, "outside.example."),
	)}, 0)
	res := resolve(t, snap, "foo.vendor.example.", model.TypeA, overlayZoneID)
	if !res.Fallthrough {
		t.Fatal("overlay wildcard CNAME outside must fall through")
	}
	wantOwner(t, res, model.TypeCNAME, "foo.vendor.example.")
}

func TestCNAMEDepthCap(t *testing.T) {
	const cap = 8
	recs := make([]model.Record, 0, cap+2)
	for i := 0; i < cap+1; i++ {
		next := fmt.Sprintf("n%d.lab.example.net.", i+1)
		recs = append(recs, rec(fmt.Sprintf("c%d", i), fmt.Sprintf("n%d", i), model.TypeCNAME, time.Second, next))
	}
	recs = append(recs, rec("final", "n9", model.TypeA, time.Second, "192.0.2.9"))
	snap := snapOf(t, []model.Zone{authZone(recs...)}, cap)

	over := resolve(t, snap, "n0.lab.example.net.", model.TypeA, authZoneID)
	if over.RCode != model.RCodeServFail {
		t.Fatalf("depth+1 rcode=%s", over.RCode)
	}
	if over.AA || over.AD || over.CD {
		t.Fatalf("SERVFAIL must not set AA/AD/CD: %+v", over)
	}

	// Cap of 8 follows 8 CNAMEs (n0..n7 → n8). n8 is still CNAME to n9; that
	// 9th hop is the one that SERVFAILs above. A chain of exactly 8 hops
	// that lands on A must succeed.
	okRecs := make([]model.Record, 0, cap+1)
	for i := 0; i < cap; i++ {
		next := fmt.Sprintf("ok%d.lab.example.net.", i+1)
		okRecs = append(okRecs, rec(fmt.Sprintf("okc%d", i), fmt.Sprintf("ok%d", i), model.TypeCNAME, time.Second, next))
	}
	okRecs = append(okRecs, rec("oka", "ok8", model.TypeA, time.Second, "192.0.2.8"))
	okSnap := snapOf(t, []model.Zone{authZone(okRecs...)}, cap)
	ok := resolve(t, okSnap, "ok0.lab.example.net.", model.TypeA, authZoneID)
	wantRCode(t, ok, model.RCodeNoError)
	wantData(t, ok, model.TypeA, "192.0.2.8")
	if len(ok.Answers) != cap+1 {
		t.Fatalf("answers=%d want %d", len(ok.Answers), cap+1)
	}
}

func TestZeroCNAMEDepthFallsBackToDefaultNotUnlimited(t *testing.T) {
	// 9 CNAMEs then A. Zero Defaults.CNAMEDepth must use DefaultCNAMEDepth (8),
	// not follow the entire chain.
	recs := make([]model.Record, 0, 11)
	for i := 0; i < 9; i++ {
		next := fmt.Sprintf("z%d.lab.example.net.", i+1)
		recs = append(recs, rec(fmt.Sprintf("z%d", i), fmt.Sprintf("z%d", i), model.TypeCNAME, time.Second, next))
	}
	recs = append(recs, rec("za", "z9", model.TypeA, time.Second, "192.0.2.9"))
	idx := mustCompile(t, []model.Zone{authZone(recs...)})
	snap := &snapshot.Snapshot{
		Zones:      idx,
		Defaults:   snapshot.DefaultsView{CNAMEDepth: 0, NegativeTTL: 10 * time.Second},
		Generation: 1,
	}
	res := resolve(t, snap, "z0.lab.example.net.", model.TypeA, authZoneID)
	if res.RCode != model.RCodeServFail {
		t.Fatalf("zero depth must cap at %d, rcode=%s answers=%d", model.DefaultCNAMEDepth, res.RCode, len(res.Answers))
	}
}

func TestRuntimeCNAMELoopSERVFAIL(t *testing.T) {
	// Wildcard CNAME pointing at a name that synthesizes the same wildcard
	// cannot be rejected at compile time (exact-graph loop detector).
	snap := snapOf(t, []model.Zone{authZone(
		rec("wc", "*", model.TypeCNAME, time.Second, "loop.lab.example.net."),
	)}, 4)
	res := resolve(t, snap, "start.lab.example.net.", model.TypeA, authZoneID)
	if res.RCode != model.RCodeServFail {
		t.Fatalf("loop rcode=%s answers=%+v", res.RCode, res.Answers)
	}
}

func TestOverlayCNAMEChainThenOutsideCountsTowardDepth(t *testing.T) {
	recs := []model.Record{
		rec("c0", "c0", model.TypeCNAME, time.Second, "c1.vendor.example."),
		rec("c1", "c1", model.TypeCNAME, time.Second, "outside.example."),
	}
	snap := snapOf(t, []model.Zone{overlayZone(recs...)}, 8)
	res := resolve(t, snap, "c0.vendor.example.", model.TypeA, overlayZoneID)
	if !res.Fallthrough {
		t.Fatal("expected fallthrough after overlay CNAME leaves local data")
	}
	if len(res.Answers) != 2 {
		t.Fatalf("answers=%+v", res.Answers)
	}
}
