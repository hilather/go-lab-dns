package forwarder

import (
	"errors"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestCompileNilAndEmpty(t *testing.T) {
	idx, err := Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.Select("a.example."); ok {
		t.Fatal("empty selected")
	}
	idx, err = Compile(&model.State{})
	if err != nil {
		t.Fatal(err)
	}
	if idx.ByID == nil || idx.Pools == nil {
		t.Fatal("maps must be non-nil")
	}
}

func TestCompileLongestSuffixAndDefaultDot(t *testing.T) {
	idx, err := Compile(&model.State{Spec: model.Spec{Forwarding: model.ForwardingSpec{
		Pools: []model.UpstreamPool{
			{ID: "p", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{
				{ID: "u", Endpoint: "10.0.0.1:53", Transport: model.TransportUDP},
			}},
		},
		Policies: []model.ForwardingPolicy{
			{ID: "def", Suffix: ".", UpstreamPool: "p"},
			{ID: "corp", Suffix: "corp.example.net.", UpstreamPool: "p"},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := idx.Select("a.corp.example.net.")
	if !ok || got != "corp" {
		t.Fatalf("got=%s ok=%v", got, ok)
	}
	got, ok = idx.Select("other.")
	if !ok || got != "def" {
		t.Fatalf("default=%s ok=%v", got, ok)
	}
}

func TestCompileFailClosed(t *testing.T) {
	_, err := Compile(&model.State{Spec: model.Spec{Forwarding: model.ForwardingSpec{
		Policies: []model.ForwardingPolicy{{ID: "p", Suffix: ".", UpstreamPool: "missing"}},
	}}})
	if !errors.Is(err, ErrInvalidForwarding) {
		t.Fatalf("missing pool err=%v", err)
	}
	_, err = Compile(&model.State{Spec: model.Spec{Forwarding: model.ForwardingSpec{
		Pools: []model.UpstreamPool{{ID: "p", Strategy: "fancy", Upstreams: []model.Upstream{
			{ID: "u", Endpoint: "10.0.0.1:53", Transport: model.TransportUDP},
		}}},
	}}})
	if !errors.Is(err, ErrInvalidForwarding) {
		t.Fatalf("strategy err=%v", err)
	}
	_, err = Compile(&model.State{Spec: model.Spec{Forwarding: model.ForwardingSpec{
		Pools: []model.UpstreamPool{{ID: "p", Strategy: model.StrategyOrdered, Upstreams: []model.Upstream{
			{ID: "u", Endpoint: "10.0.0.1:53", Transport: "dot"},
		}}},
	}}})
	if !errors.Is(err, ErrInvalidForwarding) {
		t.Fatalf("dot transport err=%v", err)
	}
}
