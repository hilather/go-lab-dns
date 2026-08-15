package dnsquery

import (
	"net"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestPacketLevelLocalAndRefused(t *testing.T) {
	st := loadPack(t)
	up := startQueryFake(t)
	rewriteUpstreams(st, up.UDPAddr())
	snap := compileSnap(t, st)
	store := snapshot.NewStore()
	store.Swap(snap)
	h := NewOpts(Opts{Store: store})

	srv, err := dnsserver.New(dnsserver.Config{
		UDPAddr: "127.0.0.1:0",
		TCPAddr: "127.0.0.1:0",
		Handler: h,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() {
		_ = srv.Shutdown(t.Context())
	})

	q, err := dnswire.PackQuery(7, model.Query{
		Name: "ns1.lab.example.net.", Type: model.TypeA, Class: model.ClassIN, RD: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	local := exchangeUDP(t, srv.UDPAddr(), q)
	msg, err := dnswire.UnpackUpstream(local)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RCode != model.RCodeNoError || !msg.AA || msg.RA {
		t.Fatalf("local flags rcode=%s AA=%v RA=%v", msg.RCode, msg.AA, msg.RA)
	}

	before := up.Packets.Load()
	q2, err := dnswire.PackQuery(8, model.Query{
		Name: "only.forwarded.example.", Type: model.TypeA, Class: model.ClassIN, RD: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := exchangeUDP(t, srv.UDPAddr(), q2)
	msg2, err := dnswire.UnpackUpstream(ref)
	if err != nil {
		t.Fatal(err)
	}
	if msg2.RCode != model.RCodeRefused || msg2.RA {
		t.Fatalf("refused rcode=%s RA=%v", msg2.RCode, msg2.RA)
	}
	if up.Packets.Load() != before {
		t.Fatal("packet path must not reach upstream for unmatched client")
	}
}

func exchangeUDP(t *testing.T, addr net.Addr, payload []byte) []byte {
	t.Helper()
	c, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	testutil.MustClose(t, c)
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf[:n]
}
