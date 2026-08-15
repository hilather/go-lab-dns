package resolver

import (
	"errors"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestExactAllStructuredTypes(t *testing.T) {
	const ttl = 30 * time.Second
	zone := authZone(
		rec("a", "a", model.TypeA, ttl, "192.0.2.10"),
		rec("aaaa", "aaaa", model.TypeAAAA, ttl, "2001:db8::10"),
		rec("txt", "txt", model.TypeTXT, ttl, `"hello"`),
		rec("mx", "mx", model.TypeMX, ttl, "10 mail.lab.example.net."),
		rec("srv", "srv", model.TypeSRV, ttl, "0 1 5060 sip.lab.example.net."),
		rec("ptr", "1.2.0.192.in-addr.arpa.lab.example.net.", model.TypePTR, ttl, "a.lab.example.net."),
		rec("caa", "caa", model.TypeCAA, ttl, `0 issue "letsencrypt.org"`),
		rec("svcb", "svcb", model.TypeSVCB, ttl, "1 svc.lab.example.net."),
		rec("https", "https", model.TypeHTTPS, ttl, "1 ."),
	)
	snap := snapOf(t, []model.Zone{zone}, 0)

	cases := []struct {
		name string
		typ  model.RRType
		data string
	}{
		{"a.lab.example.net.", model.TypeA, "192.0.2.10"},
		{"aaaa.lab.example.net.", model.TypeAAAA, "2001:db8::10"},
		{"txt.lab.example.net.", model.TypeTXT, `"hello"`},
		{"mx.lab.example.net.", model.TypeMX, "10 mail.lab.example.net."},
		{"srv.lab.example.net.", model.TypeSRV, "0 1 5060 sip.lab.example.net."},
		{"1.2.0.192.in-addr.arpa.lab.example.net.", model.TypePTR, "a.lab.example.net."},
		{"caa.lab.example.net.", model.TypeCAA, `0 issue "letsencrypt.org"`},
		{"svcb.lab.example.net.", model.TypeSVCB, "1 svc.lab.example.net."},
		{"https.lab.example.net.", model.TypeHTTPS, "1 ."},
	}
	for _, tc := range cases {
		t.Run(string(tc.typ), func(t *testing.T) {
			res := resolve(t, snap, tc.name, tc.typ, authZoneID)
			wantRCode(t, res, model.RCodeNoError)
			wantData(t, res, tc.typ, tc.data)
			if res.Source != model.SourceExact {
				t.Fatalf("source=%s", res.Source)
			}
			if !res.AA || res.AD || res.CD || res.RA || res.Fallthrough {
				t.Fatalf("flags AA=%v AD=%v CD=%v RA=%v ft=%v", res.AA, res.AD, res.CD, res.RA, res.Fallthrough)
			}
		})
	}

	res := resolve(t, snap, "lab.example.net.", model.TypeNS, authZoneID)
	wantRCode(t, res, model.RCodeNoError)
	wantData(t, res, model.TypeNS, "ns1.lab.example.net.")

	res = resolve(t, snap, "lab.example.net.", model.TypeSOA, authZoneID)
	wantRCode(t, res, model.RCodeNoError)
	if len(res.Answers) != 1 || res.Answers[0].Type != model.TypeSOA {
		t.Fatalf("soa answers=%+v", res.Answers)
	}
	if res.Answers[0].Data == "" || res.Answers[0].Data[:3] == "auto" {
		t.Fatalf("auto serial not materialized: %q", res.Answers[0].Data)
	}
}

func TestAuthoritativeNXDOMAINVersusNODATA(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
		rec("txt", "branch.tools", model.TypeTXT, time.Second, `"exists"`),
	)}, 0)

	nx := resolve(t, snap, "no.such.lab.example.net.", model.TypeA, authZoneID)
	wantRCode(t, nx, model.RCodeNXDomain)
	if nx.Fallthrough || nx.Source != model.SourceNegative || !nx.AA {
		t.Fatalf("nx flags %+v", nx)
	}
	if len(nx.Authority) != 1 || nx.Authority[0].Type != model.TypeSOA {
		t.Fatalf("nx soa=%+v", nx.Authority)
	}

	nodata := resolve(t, snap, "ns1.lab.example.net.", model.TypeAAAA, authZoneID)
	wantRCode(t, nodata, model.RCodeNoError)
	if nodata.Fallthrough || nodata.Source != model.SourceNegative || !nodata.AA {
		t.Fatalf("nodata flags %+v", nodata)
	}
	if len(nodata.Answers) != 0 {
		t.Fatalf("nodata answers=%+v", nodata.Answers)
	}
	if len(nodata.Authority) != 1 || nodata.Authority[0].Type != model.TypeSOA {
		t.Fatalf("nodata soa=%+v", nodata.Authority)
	}

	ent := resolve(t, snap, "tools.lab.example.net.", model.TypeA, authZoneID)
	wantRCode(t, ent, model.RCodeNoError)
	if ent.Source != model.SourceNegative || ent.Fallthrough {
		t.Fatalf("ENT should be NODATA, got %+v", ent)
	}
}

func TestOverlayFallthroughNeverNXDOMAIN(t *testing.T) {
	snap := snapOf(t, []model.Zone{overlayZone(
		rec("hit", "special-api", model.TypeA, time.Second, "10.42.0.30"),
		rec("txt", "only-txt", model.TypeTXT, time.Second, `"x"`),
	)}, 0)

	hit := resolve(t, snap, "special-api.vendor.example.", model.TypeA, overlayZoneID)
	wantRCode(t, hit, model.RCodeNoError)
	if hit.AA || hit.Fallthrough || hit.Source != model.SourceExact {
		t.Fatalf("overlay hit %+v", hit)
	}
	wantData(t, hit, model.TypeA, "10.42.0.30")

	miss := resolve(t, snap, "other.vendor.example.", model.TypeA, overlayZoneID)
	if !miss.Fallthrough || miss.RCode != model.RCodeNoError || miss.Source != model.SourceFallthrough {
		t.Fatalf("overlay miss %+v", miss)
	}
	if miss.AA || len(miss.Answers) != 0 {
		t.Fatalf("overlay miss must not be authoritative or answer: %+v", miss)
	}

	wrongType := resolve(t, snap, "only-txt.vendor.example.", model.TypeA, overlayZoneID)
	if !wrongType.Fallthrough {
		t.Fatalf("overlay wrong-type must fall through, got %+v", wrongType)
	}

	ent := resolve(t, snap, "vendor.example.", model.TypeAAAA, overlayZoneID)
	if !ent.Fallthrough {
		t.Fatalf("overlay apex without type must fall through, got %+v", ent)
	}
}

func TestPreselectedZoneIDNotRediscovered(t *testing.T) {
	snap := snapOf(t, []model.Zone{
		authZone(rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53")),
		{
			ID:   "tools-overlay",
			Name: "tools.lab.example.net.",
			Mode: model.ZoneModeOverlay,
			Records: []model.Record{
				rec("w", "*", model.TypeA, time.Second, "10.42.0.20"),
			},
		},
	}, 0)

	// Orchestrator selected the parent auth zone; do not consult the overlay.
	res := resolve(t, snap, "alpha.tools.lab.example.net.", model.TypeA, authZoneID)
	wantRCode(t, res, model.RCodeNXDomain)
	if res.Fallthrough || res.ZoneID != authZoneID {
		t.Fatalf("used overlay or fallthrough: %+v", res)
	}

	// Name outside the selected overlay suffix: do not NXDOMAIN.
	out := resolve(t, snap, "ns1.lab.example.net.", model.TypeA, "tools-overlay")
	if !out.Fallthrough {
		t.Fatalf("out-of-zone with overlay id must fall through: %+v", out)
	}
}

func TestEmptyZoneIDIsFallthrough(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"))}, 0)
	res := resolve(t, snap, "ns1.lab.example.net.", model.TypeA, "")
	if !res.Fallthrough || res.RCode != model.RCodeNoError {
		t.Fatalf("%+v", res)
	}
}

func TestUnknownZoneID(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone()}, 0)
	_, err := Resolve(t.Context(), snap, model.Query{Name: "lab.example.net.", Type: model.TypeA}, "nope")
	if !errors.Is(err, ErrUnknownZone) {
		t.Fatalf("err=%v", err)
	}
}

func TestNilSnapshot(t *testing.T) {
	_, err := Resolve(t.Context(), nil, model.Query{Name: "x.", Type: model.TypeA}, authZoneID)
	if !errors.Is(err, ErrNilSnapshot) {
		t.Fatalf("err=%v", err)
	}
}

func TestPackSampleFixture(t *testing.T) {
	st, err := config.LoadFile(repoRoot(t) + "/testdata/config/valid/pack-sample.yaml")
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	snap := &snapshot.Snapshot{
		Zones:      idx,
		Defaults:   snapshot.DefaultsView{TTL: 30 * time.Second, NegativeTTL: 10 * time.Second, CNAMEDepth: 8},
		Revision:   "sha256:pack",
		Generation: 1,
	}

	a := resolve(t, snap, "ns1.lab.example.net.", model.TypeA, "lab-zone")
	wantData(t, a, model.TypeA, "10.42.0.53")

	w := resolve(t, snap, "alpha.tools.lab.example.net.", model.TypeA, "lab-zone")
	wantRCode(t, w, model.RCodeNoError)
	wantData(t, w, model.TypeA, "10.42.0.20")
	wantOwner(t, w, model.TypeA, "alpha.tools.lab.example.net.")
	if w.Source != model.SourceWildcard || w.WildcardSource == nil || *w.WildcardSource != "tools-wildcard-a" {
		t.Fatalf("wildcard explain %+v", w)
	}

	c := resolve(t, snap, "grafana.tools.lab.example.net.", model.TypeA, "lab-zone")
	wantData(t, c, model.TypeCNAME, "gateway.lab.example.net.")
	if c.Fallthrough {
		t.Fatal("auth CNAME out-of-local-data must not fall through")
	}

	o := resolve(t, snap, "special-api.vendor.example.", model.TypeA, "vendor-overlay")
	wantData(t, o, model.TypeA, "10.42.0.30")
	if o.AA {
		t.Fatal("overlay hit must not set AA")
	}

	ft := resolve(t, snap, "other.vendor.example.", model.TypeA, "vendor-overlay")
	if !ft.Fallthrough {
		t.Fatalf("overlay miss: %+v", ft)
	}
}
