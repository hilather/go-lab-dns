package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
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
	want := []TargetKind{
		"zone", "record", "forwardingPolicy", "upstreamPool", "upstream",
		"clientGroup", "chaosPolicy", "chaosSafety", "cache", "defaults",
		"listeners", "access", "observability", "management", "chaosActivation",
	}
	if len(AllTargetKinds) != len(want) {
		t.Fatalf("AllTargetKinds len=%d, want %d", len(AllTargetKinds), len(want))
	}
	consts := stringConstsOfType(t, ".", "TargetKind")
	if len(consts) != len(AllTargetKinds) {
		t.Fatalf("TargetKind constants=%v AllTargetKinds=%v (catalog drift)", consts, AllTargetKinds)
	}
	seen := map[TargetKind]bool{}
	for i, got := range AllTargetKinds {
		if got != want[i] {
			t.Fatalf("AllTargetKinds[%d]=%q, want %q", i, got, want[i])
		}
		if !consts[string(got)] {
			t.Fatalf("AllTargetKinds contains %q with no matching const", got)
		}
		if seen[got] {
			t.Fatalf("duplicate target %q", got)
		}
		seen[got] = true
	}
}

func TestOpKindsLocked(t *testing.T) {
	want := []OpKind{"add", "update", "remove"}
	if len(AllOpKinds) != len(want) {
		t.Fatalf("AllOpKinds=%v, want %v", AllOpKinds, want)
	}
	for i, k := range want {
		if AllOpKinds[i] != k {
			t.Fatalf("AllOpKinds[%d]=%q, want %q", i, AllOpKinds[i], k)
		}
	}
	consts := stringConstsOfType(t, ".", "OpKind")
	if len(consts) != len(want) {
		t.Fatalf("OpKind constants=%v want %v", consts, want)
	}
}

func TestTransportsLockedNoDoT(t *testing.T) {
	if len(AllTransports) != 2 || AllTransports[0] != TransportUDP || AllTransports[1] != TransportTCP {
		t.Fatalf("AllTransports=%v, want [udp tcp]", AllTransports)
	}
	consts := stringConstsOfType(t, ".", "Transport")
	if len(consts) != 2 || !consts["udp"] || !consts["tcp"] {
		t.Fatalf("Transport constants=%v; DoT or extra transport leaked", consts)
	}
	for name := range consts {
		if name == "dot" || name == "tls" || name == "DoT" {
			t.Fatalf("forbidden transport constant %q", name)
		}
	}
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

func stringConstsOfType(t *testing.T, dir, typeName string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
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
