package model

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	"github.com/hilather/go-lab-dns/internal/testutil/goparse"
)

func TestFirstGARRTypesLocked(t *testing.T) {
	want := []RRType{
		"A", "AAAA", "CNAME", "TXT", "MX", "SRV",
		"PTR", "CAA", "NS", "SOA", "SVCB", "HTTPS",
	}
	if len(FirstGARRTypes) != len(want) {
		t.Fatalf("FirstGARRTypes len=%d, want %d (half-migration?)", len(FirstGARRTypes), len(want))
	}
	seen := map[RRType]bool{}
	for i, got := range FirstGARRTypes {
		if got != want[i] {
			t.Fatalf("FirstGARRTypes[%d]=%q, want %q", i, got, want[i])
		}
		if seen[got] {
			t.Fatalf("duplicate RRType %q", got)
		}
		seen[got] = true
	}
}

func TestTargetKindsLocked(t *testing.T) {
	assertClosedStringEnum(t, "TargetKind", AllTargetKinds, []TargetKind{
		"zone", "record", "forwardingPolicy", "upstreamPool", "upstream",
		"clientGroup", "chaosPolicy", "chaosSafety", "cache", "defaults",
		"listeners", "access", "observability", "management", "ui", "chaosActivation",
	})
}

func TestOpKindsLocked(t *testing.T) {
	assertClosedStringEnum(t, "OpKind", AllOpKinds, []OpKind{"add", "update", "remove"})
}

func TestTransportsLockedNoDoT(t *testing.T) {
	assertClosedStringEnum(t, "Transport", AllTransports, []Transport{TransportUDP, TransportTCP})
	for name := range stringConstsOfType(t, ".", "Transport") {
		if name == "dot" || name == "tls" || name == "DoT" {
			t.Fatalf("forbidden transport constant %q", name)
		}
	}
}

func TestZoneModesLocked(t *testing.T) {
	assertClosedStringEnum(t, "ZoneMode", AllZoneModes, []ZoneMode{
		ZoneModeAuthoritative, ZoneModeOverlay,
	})
	for name := range stringConstsOfType(t, ".", "ZoneMode") {
		if name == "recursive" || name == "forwarding" {
			t.Fatalf("forbidden zone mode %q", name)
		}
	}
}

func TestPoolStrategiesLocked(t *testing.T) {
	assertClosedStringEnum(t, "PoolStrategy", AllPoolStrategies, []PoolStrategy{
		StrategyOrdered, StrategyRoundRobin, StrategyRandom, StrategyHealthAware,
	})
}

func TestUnknownClientModesLocked(t *testing.T) {
	assertClosedStringEnum(t, "UnknownClientMode", AllUnknownClientModes, []UnknownClientMode{
		UnknownClientRefuseForward,
	})
}

func TestUnknownClientAndCNAMEDepthDefaults(t *testing.T) {
	if UnknownClientRefuseForward != "refuse-forward" {
		t.Fatalf("UnknownClientRefuseForward=%q", UnknownClientRefuseForward)
	}
	if DefaultCNAMEDepth != 8 {
		t.Fatalf("DefaultCNAMEDepth=%d, want 8", DefaultCNAMEDepth)
	}
	if APIVersionV1Alpha1 != "labdns.dev/v1alpha1" || KindLabDNS != "LabDNS" {
		t.Fatalf("api %q kind %q", APIVersionV1Alpha1, KindLabDNS)
	}
}

func TestAllowForwardMaterializedDefault(t *testing.T) {
	if DefaultAllowForward != true {
		t.Fatalf("DefaultAllowForward=%v, want true (materialized default when a group exists)", DefaultAllowForward)
	}
	var zero ClientGroup
	if zero.AllowForward {
		t.Fatal("unmaterialized ClientGroup.AllowForward zero value is true; want false")
	}
}

func TestUIEnabledMaterializedDefault(t *testing.T) {
	if DefaultUIEnabled != true {
		t.Fatalf("DefaultUIEnabled=%v, want true (materialized default when spec.ui is omitted)", DefaultUIEnabled)
	}
	var zero UISpec
	if zero.Enabled {
		t.Fatal("unmaterialized UISpec.Enabled zero value is true; want false")
	}
}

func assertClosedStringEnum[T ~string](t *testing.T, typeName string, all, want []T) {
	t.Helper()
	if len(all) != len(want) {
		t.Fatalf("%s All len=%d, want %d (half-migration?)\n All=%v\n want=%v", typeName, len(all), len(want), all, want)
	}
	consts := stringConstsOfType(t, ".", typeName)
	if len(consts) != len(all) {
		t.Fatalf("%s typed constants=%v All=%v (extra typed constant?)", typeName, consts, all)
	}
	seen := map[T]bool{}
	for i, got := range all {
		if got != want[i] {
			t.Fatalf("%s[%d]=%q, want %q", typeName, i, got, want[i])
		}
		if !consts[string(got)] {
			t.Fatalf("%s All contains %q with no matching const", typeName, got)
		}
		if seen[got] {
			t.Fatalf("duplicate %s %q", typeName, got)
		}
		seen[got] = true
	}
}

func stringConstsOfType(t *testing.T, dir, typeName string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := goparse.ParseDir(fset, dir, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	out := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.Name == "model_test" {
			continue
		}
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				gen, ok := d.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs := spec.(*ast.ValueSpec)
					if vs.Type != nil {
						if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != typeName {
							continue
						}
					} else {
						continue
					}
					for _, v := range vs.Values {
						lit, ok := v.(*ast.BasicLit)
						if !ok || lit.Kind != token.STRING {
							continue
						}
						s, err := strconv.Unquote(lit.Value)
						if err != nil {
							t.Fatalf("unquote %s: %v", lit.Value, err)
						}
						out[s] = true
					}
				}
			}
		}
	}
	return out
}
