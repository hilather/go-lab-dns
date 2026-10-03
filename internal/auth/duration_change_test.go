package auth

import (
	"github.com/hilather/go-lab-dns/internal/model"
	"testing"
)

func TestAuthorizationIgnoresNonSecurityDurationRepresentations(t *testing.T) {
	st := &model.State{}
	st.Spec.Zones = []model.Zone{{ID: "lab", Name: "lab.example.", Mode: model.ZoneModeOverlay}}
	editor := Actor{Role: RoleDNSEditor}
	for _, op := range []model.Operation{
		{Op: model.OpAdd, Target: model.Target{Kind: model.TargetRecord, ID: "ordinary", ZoneID: "lab"}, Value: []byte(`{"id":"ordinary","owner":"ordinary","type":"A","ttl":"30s","values":["192.0.2.1"]}`)},
		{Op: model.OpAdd, Target: model.Target{Kind: model.TargetZone, ID: "new"}, Value: []byte(`{"id":"new","name":"new.example.","mode":"overlay","records":[{"id":"r","owner":"ordinary","type":"A","ttl":"30s","values":["192.0.2.1"]}]}`)},
	} {
		if err := AuthorizeChange(editor, []model.Operation{op}, st); err != nil {
			t.Fatalf("ordinary duration string denied: %v", err)
		}
	}
	designer := Actor{Role: RoleChaosDesigner}
	op := model.Operation{Op: model.OpAdd, Target: model.Target{Kind: model.TargetChaosPolicy, ID: "p"}, Value: []byte(`{"id":"p","enabled":false,"safetyClass":"low","selector":{"timeBucket":"1s"},"outcomes":[{"id":"o","weight":1,"actions":[{"type":"delay","duration":"100ms"}]}]}`)}
	if err := AuthorizeChange(designer, []model.Operation{op}, st); err != nil {
		t.Fatalf("disabled duration policy denied: %v", err)
	}
}
