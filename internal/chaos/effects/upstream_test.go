package effects

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestExchangeOpts(t *testing.T) {
	opts := Exchange(chaos.ActionPlan{Upstream: chaos.UpstreamPlan{
		Force:          "u2",
		Timeout:        true,
		TransportError: true,
		Failover:       true,
		SyntheticRCode: model.RCodeServFail,
		Unavailable:    []model.UpstreamID{"u1"},
	}}, nil)
	if opts.ForceUpstream != "u2" || !opts.ForceTimeout || !opts.ForceTransportError || !opts.ForceFailover {
		t.Fatalf("%+v", opts)
	}
	if !opts.Unavailable["u1"] || opts.SyntheticRCode != model.RCodeServFail {
		t.Fatalf("%+v", opts)
	}
}
