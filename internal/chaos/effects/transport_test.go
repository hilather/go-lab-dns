package effects

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestHintMapping(t *testing.T) {
	cases := []struct {
		name string
		hint string
		tr   model.Transport
		hold time.Duration
		want dnsserver.TransportHint
	}{
		{"udp-drop", "drop", model.TransportUDP, 0, dnsserver.HintDrop},
		{"tcp-drop-hold", "drop", model.TransportTCP, 0, dnsserver.HintHoldThenClose},
		{"udp-tc", "truncate", model.TransportUDP, 0, dnsserver.HintTruncate},
		{"tcp-close", "tcp-close", model.TransportTCP, 0, dnsserver.HintTCPClose},
		{"tcp-close-hold", "tcp-close", model.TransportTCP, time.Second, dnsserver.HintHoldThenClose},
		{"tcp-reset", "tcp-reset", model.TransportTCP, 0, dnsserver.HintTCPReset},
		{"send", "", model.TransportUDP, 0, dnsserver.HintSend},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Hint(chaos.ActionPlan{TransportHint: tc.hint, Hold: tc.hold}, tc.tr, nil)
			if got != tc.want {
				t.Fatalf("got=%s want=%s", got, tc.want)
			}
		})
	}
}
