package forwarder

import (
	"github.com/hilather/go-lab-dns/internal/model"
	"testing"
)

func TestSyntheticUpstreamFaultsDoNotPoisonHealth(t *testing.T) {
	up := startFake(t)
	snap := snapOne(t, up.UDPAddr(), model.TransportUDP, model.FailoverSpec{})
	for _, options := range []ExchangeOpts{{ForceTimeout: true}, {ForceTransportError: true}} {
		runtime := NewRuntime(nil, nil, nil, nil)
		for range DefaultFailThreshold {
			result, err := runtime.ExchangeOpts(t.Context(), snap, query("x.example."), "pol", options)
			if err != nil || result.RCode != model.RCodeServFail {
				t.Fatalf("synthetic failure=%+v %v", result, err)
			}
		}
		down, failures := runtime.Health.Snapshot("u1")
		if down || failures != 0 {
			t.Fatalf("synthetic fault poisoned real health: down=%v failures=%d", down, failures)
		}
	}
}
