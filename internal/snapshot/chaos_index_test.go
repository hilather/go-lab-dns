package snapshot

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestChaosIndexZero(t *testing.T) {
	var idx ChaosIndex
	if idx.Compiled() {
		t.Fatal("zero compiled")
	}
	if _, ok := idx.Lookup("x"); ok {
		t.Fatal("lookup")
	}
	if got := idx.Candidates(ChaosMatch{Owner: "a."}); got != nil {
		t.Fatalf("candidates=%v", got)
	}
}

func TestChaosIndexCandidatesOrder(t *testing.T) {
	idx := ChaosIndex{
		ByID: map[model.PolicyID]*CompiledChaos{
			"r": {Policy: model.ChaosPolicy{ID: "r"}, Precedence: ChaosPrecRecord},
			"z": {Policy: model.ChaosPolicy{ID: "z"}, Precedence: ChaosPrecZone},
			"g": {Policy: model.ChaosPolicy{ID: "g"}, Precedence: ChaosPrecGlobal},
		},
		ByRecord: map[model.RecordID][]model.PolicyID{"rid": {"r"}},
		ByZone:   map[model.ZoneID][]model.PolicyID{"zid": {"z"}},
		Global:   []model.PolicyID{"g"},
	}
	got := idx.Candidates(ChaosMatch{RecordIDs: []model.RecordID{"rid"}, ZoneID: "zid"})
	if len(got) != 3 || got[0].Policy.ID != "r" || got[1].Policy.ID != "z" || got[2].Policy.ID != "g" {
		t.Fatalf("order=%v", idsOf(got))
	}
}

func idsOf(ps []*CompiledChaos) []model.PolicyID {
	out := make([]model.PolicyID, len(ps))
	for i, p := range ps {
		out[i] = p.Policy.ID
	}
	return out
}
