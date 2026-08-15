package resolver

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestLocalFlagMatrix(t *testing.T) {
	snap := snapOf(t, []model.Zone{
		authZone(
			rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
		),
		overlayZone(
			rec("o", "special-api", model.TypeA, time.Second, "10.42.0.30"),
		),
	}, 0)

	type tc struct {
		name   string
		qname  string
		zone   model.ZoneID
		rd, cd bool
		wantAA bool
		wantFT bool
		rcode  model.RCode
	}
	cases := []tc{
		{name: "auth-RD0-CD0", qname: "ns1.lab.example.net.", zone: authZoneID, wantAA: true, rcode: model.RCodeNoError},
		{name: "auth-RD1-CD1", qname: "ns1.lab.example.net.", zone: authZoneID, rd: true, cd: true, wantAA: true, rcode: model.RCodeNoError},
		{name: "auth-nx-RD1-CD1", qname: "no.lab.example.net.", zone: authZoneID, rd: true, cd: true, wantAA: true, rcode: model.RCodeNXDomain},
		{name: "overlay-hit-RD0-CD0", qname: "special-api.vendor.example.", zone: overlayZoneID, rcode: model.RCodeNoError},
		{name: "overlay-hit-RD1-CD1", qname: "special-api.vendor.example.", zone: overlayZoneID, rd: true, cd: true, rcode: model.RCodeNoError},
		{name: "overlay-miss-RD1-CD1", qname: "miss.vendor.example.", zone: overlayZoneID, rd: true, cd: true, wantFT: true, rcode: model.RCodeNoError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Resolve(t.Context(), snap, model.Query{
				Name:  model.Name(tc.qname),
				Type:  model.TypeA,
				Class: model.ClassIN,
				RD:    tc.rd,
				CD:    tc.cd,
			}, tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			if res.RCode != tc.rcode {
				t.Fatalf("rcode=%s", res.RCode)
			}
			if res.AA != tc.wantAA {
				t.Fatalf("AA=%v want %v", res.AA, tc.wantAA)
			}
			if res.Fallthrough != tc.wantFT {
				t.Fatalf("fallthrough=%v want %v", res.Fallthrough, tc.wantFT)
			}
			if res.AD {
				t.Fatal("must never forge AD on local data")
			}
			if res.CD {
				t.Fatal("local answers must clear CD")
			}
			if res.RA {
				t.Fatal("resolver must not set RA")
			}
		})
	}
}
