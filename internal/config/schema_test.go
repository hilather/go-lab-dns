package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestSchemaPublished(t *testing.T) {
	b, err := SchemaBytes()
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	if root["$id"] != "https://labdns.dev/schema/v1alpha1" {
		t.Fatalf("$id=%v", root["$id"])
	}
	if root["additionalProperties"] != false {
		t.Fatal("root additionalProperties must be false")
	}
	defs, _ := root["$defs"].(map[string]any)
	required := []string{
		"spec", "access", "clientGroup", "defaults", "zone", "record",
		"forwarding", "forwardingPolicy", "upstreamPool", "upstream",
		"cache", "chaos", "chaosPolicy", "chaosSelector", "chaosAction",
		"management", "auth",
	}
	for _, name := range required {
		def, ok := defs[name].(map[string]any)
		if !ok {
			t.Fatalf("missing $defs.%s", name)
			continue
		}
		if def["additionalProperties"] != false {
			t.Fatalf("$defs.%s additionalProperties=%v want false", name, def["additionalProperties"])
		}
	}
	access := defs["access"].(map[string]any)
	props := access["properties"].(map[string]any)
	uc := props["unknownClient"].(map[string]any)
	if uc["const"] != "refuse-forward" {
		t.Fatalf("unknownClient const=%v", uc["const"])
	}
}

func TestSchemaListsModelJSONFields(t *testing.T) {
	b, err := SchemaBytes()
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	schemaKeys := collectSchemaPropertyNames(raw)
	for _, sample := range []any{
		model.State{}, model.Spec{}, model.AccessSpec{}, model.ClientGroup{},
		model.DefaultsSpec{}, model.Zone{}, model.Record{}, model.ForwardingSpec{},
		model.ForwardingPolicy{}, model.UpstreamPool{}, model.Upstream{},
		model.CacheSpec{}, model.ChaosSpec{}, model.ChaosPolicy{}, model.ChaosSelector{},
		model.ChaosAction{}, model.ManagementSpec{}, model.AuthSpec{},
	} {
		rt := reflect.TypeOf(sample)
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			tag := f.Tag.Get("json")
			name, _, _ := strings.Cut(tag, ",")
			if name == "" || name == "-" {
				continue
			}
			if !schemaKeys[name] {
				t.Errorf("model %s.%s json %q missing from schema", rt.Name(), f.Name, name)
			}
		}
	}
}

func collectSchemaPropertyNames(v any) map[string]bool {
	out := map[string]bool{}
	var walk func(any)
	walk = func(x any) {
		m, ok := x.(map[string]any)
		if !ok {
			if arr, ok := x.([]any); ok {
				for _, c := range arr {
					walk(c)
				}
			}
			return
		}
		if props, ok := m["properties"].(map[string]any); ok {
			for k, child := range props {
				out[k] = true
				walk(child)
			}
		}
		for _, child := range m {
			walk(child)
		}
	}
	walk(v)
	return out
}

func TestSchemaFileExistsAtPublishedPath(t *testing.T) {
	p := filepath.Join(repoRoot(t), filepath.FromSlash(SchemaRelPath))
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestDurationFormatRoundTrip(t *testing.T) {
	cases := []time.Duration{
		0, time.Second, 30 * time.Second, time.Minute, 5 * time.Minute,
		time.Hour, 24 * time.Hour, 100 * time.Millisecond, 750 * time.Millisecond,
	}
	for _, d := range cases {
		s := FormatDuration(d)
		got, err := time.ParseDuration(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if got != d {
			t.Fatalf("%s parsed to %s want %s", s, got, d)
		}
	}
}
