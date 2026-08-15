package resolver

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// RFC 4592 / docs/02 examples plus empty-non-terminal and literal asterisk.
func TestRFC4592WildcardMatrix(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("wild", "*.tools", model.TypeA, 30*time.Second, "10.42.0.20"),
		rec("exact", "exact.tools", model.TypeA, 30*time.Second, "10.42.0.21"),
		rec("branch", "branch.tools", model.TypeTXT, 30*time.Second, `"exists"`),
	)}, 0)

	t.Run("synthesize", func(t *testing.T) {
		res := resolve(t, snap, "alpha.tools.lab.example.net.", model.TypeA, authZoneID)
		wantRCode(t, res, model.RCodeNoError)
		wantData(t, res, model.TypeA, "10.42.0.20")
		wantOwner(t, res, model.TypeA, "alpha.tools.lab.example.net.")
		if res.Source != model.SourceWildcard {
			t.Fatalf("source=%s", res.Source)
		}
		if res.WildcardSource == nil || *res.WildcardSource != "wild" {
			t.Fatalf("wildcard source=%v", res.WildcardSource)
		}
		if res.ClosestEncloser == nil || *res.ClosestEncloser != "tools.lab.example.net." {
			t.Fatalf("encloser=%v", res.ClosestEncloser)
		}
		if res.Explanation == nil || res.Explanation.WildcardSource == nil {
			t.Fatal("explanation missing wildcard source")
		}
	})

	t.Run("exact-over-wildcard", func(t *testing.T) {
		res := resolve(t, snap, "exact.tools.lab.example.net.", model.TypeA, authZoneID)
		wantData(t, res, model.TypeA, "10.42.0.21")
		if res.Source != model.SourceExact {
			t.Fatalf("source=%s", res.Source)
		}
		if res.WildcardSource != nil {
			t.Fatalf("exact must not report wildcard source")
		}
	})

	t.Run("empty-non-terminal-blocks-higher-wildcard", func(t *testing.T) {
		res := resolve(t, snap, "x.branch.tools.lab.example.net.", model.TypeA, authZoneID)
		wantRCode(t, res, model.RCodeNXDomain)
		if res.Fallthrough {
			t.Fatal("auth miss must not fall through")
		}
	})

	t.Run("multi-label-below-encloser-still-synthesizes", func(t *testing.T) {
		// RFC 4592: a.b.tools matches *.tools when b.tools does not exist.
		res := resolve(t, snap, "a.b.tools.lab.example.net.", model.TypeA, authZoneID)
		wantRCode(t, res, model.RCodeNoError)
		wantData(t, res, model.TypeA, "10.42.0.20")
		if res.Source != model.SourceWildcard {
			t.Fatalf("source=%s", res.Source)
		}
	})

	t.Run("literal-asterisk-is-exact", func(t *testing.T) {
		res := resolve(t, snap, "*.tools.lab.example.net.", model.TypeA, authZoneID)
		wantData(t, res, model.TypeA, "10.42.0.20")
		if res.Source != model.SourceExact {
			t.Fatalf("literal * must be exact, got %s", res.Source)
		}
	})

	t.Run("wildcard-wrong-type-is-NODATA", func(t *testing.T) {
		res := resolve(t, snap, "alpha.tools.lab.example.net.", model.TypeAAAA, authZoneID)
		wantRCode(t, res, model.RCodeNoError)
		if res.Source != model.SourceNegative || res.Fallthrough {
			t.Fatalf("want NODATA, got %+v", res)
		}
		if len(res.Authority) != 1 || res.Authority[0].Type != model.TypeSOA {
			t.Fatalf("soa=%+v", res.Authority)
		}
	})
}

func TestEmptyNonTerminalMatrix(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("leaf", "a.b.tools", model.TypeA, time.Second, "192.0.2.8"),
		rec("wild", "*", model.TypeA, time.Second, "192.0.2.1"),
	)}, 0)

	cases := []struct {
		qname  string
		rcode  model.RCode
		source model.Source
		data   string
	}{
		{"a.b.tools.lab.example.net.", model.RCodeNoError, model.SourceExact, "192.0.2.8"},
		{"b.tools.lab.example.net.", model.RCodeNoError, model.SourceNegative, ""},
		{"tools.lab.example.net.", model.RCodeNoError, model.SourceNegative, ""},
		{"x.b.tools.lab.example.net.", model.RCodeNXDomain, model.SourceNegative, ""},
		{"other.lab.example.net.", model.RCodeNoError, model.SourceWildcard, "192.0.2.1"},
	}
	for _, tc := range cases {
		t.Run(tc.qname, func(t *testing.T) {
			res := resolve(t, snap, tc.qname, model.TypeA, authZoneID)
			wantRCode(t, res, tc.rcode)
			if res.Source != tc.source {
				t.Fatalf("source=%s want %s", res.Source, tc.source)
			}
			if tc.data != "" {
				wantData(t, res, model.TypeA, tc.data)
			}
		})
	}
}

func TestOverlayWildcardFallthroughOnWrongType(t *testing.T) {
	snap := snapOf(t, []model.Zone{overlayZone(
		rec("w", "*", model.TypeA, time.Second, "10.42.0.30"),
	)}, 0)
	res := resolve(t, snap, "foo.vendor.example.", model.TypeTXT, overlayZoneID)
	if !res.Fallthrough {
		t.Fatalf("overlay wildcard wrong type must fall through: %+v", res)
	}
	hit := resolve(t, snap, "foo.vendor.example.", model.TypeA, overlayZoneID)
	wantData(t, hit, model.TypeA, "10.42.0.30")
	if hit.AA || hit.Source != model.SourceWildcard {
		t.Fatalf("%+v", hit)
	}
}
