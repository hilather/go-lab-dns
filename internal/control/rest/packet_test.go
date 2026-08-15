package rest

import (
	"context"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/dnsquery"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Packet-level chaos is executed by dnsquery + chaos.effects, not by REST.
func TestPacketChaosIndependentOfREST(t *testing.T) {
	path := copyNamedFixture(t, "empty-client-groups.yaml")
	st, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.Chaos.Enabled = true
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "sf", Owner: "test", Reason: "packet", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"ns1-a"}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "p", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionRCode, Value: "SERVFAIL", Phase: model.PhaseBeforeResponse},
		}}},
	}}
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	eng := chaos.NewEngine(clk, nil)
	h := dnsquery.New(store, eng, nil, nil, clk)

	q := model.Query{
		Name: "ns1.lab.example.net.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP,
	}
	resp, _, err := h.ServeDNS(context.Background(), &q)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Result().RCode != model.RCodeServFail {
		t.Fatalf("packet chaos rcode=%s (REST was never constructed)", resp.Result().RCode)
	}
}

func TestPacketChaosThenRESTEmergency(t *testing.T) {
	path := copyNamedFixture(t, "empty-client-groups.yaml")
	st, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.Chaos.Enabled = true
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "sf", Owner: "test", Reason: "packet", Enabled: true, SafetyClass: model.SafetyClassLow,
		Scope:    model.ChaosScope{RecordIDs: []model.RecordID{"ns1-a"}},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "p", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionRCode, Value: "SERVFAIL", Phase: model.PhaseBeforeResponse},
		}}},
	}}
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	eng := chaos.NewEngine(clk, nil)
	svc := app.New(app.Options{Store: store, Clock: clk, Engine: eng, BootstrapPath: path})
	dnsH := dnsquery.New(store, eng, nil, nil, clk)

	q := model.Query{
		Name: "ns1.lab.example.net.", Type: model.TypeA, Class: model.ClassIN,
		Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP,
	}
	resp, _, err := dnsH.ServeDNS(context.Background(), &q)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Result().RCode != model.RCodeServFail {
		t.Fatalf("pre-emergency rcode=%s", resp.Result().RCode)
	}

	rs, err := New(Config{Service: svc})
	if err != nil {
		t.Fatal(err)
	}
	dis := doLoopback(t, rs.Handler(), http.MethodPost, "/v1/chaos:emergency-disable", `{"reason":"packet-test"}`)
	requireStatus(t, dis, http.StatusOK)

	resp2, _, err := dnsH.ServeDNS(context.Background(), &q)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Result().RCode != model.RCodeNoError {
		t.Fatalf("post-emergency rcode=%s want NOERROR", resp2.Result().RCode)
	}
}
