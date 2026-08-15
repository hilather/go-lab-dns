package resolver

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// handler wraps Resolve. Overlay fallthrough is signaled as REFUSED so the
// packet is distinguishable from NODATA; the resolver itself does not forward.
func handler(snap *snapshot.Snapshot) dnsserver.Handler {
	return dnsserver.HandlerFunc(func(ctx context.Context, q *model.Query) (*dnsserver.Response, dnsserver.TransportHint, error) {
		if q == nil {
			return nil, dnsserver.HintDrop, nil
		}
		zid, _ := snap.Zones.Select(q.Name)
		res, err := Resolve(ctx, snap, *q, zid)
		if err != nil {
			return nil, dnsserver.HintDrop, err
		}
		if res.Fallthrough && len(res.Answers) == 0 {
			res.RCode = model.RCodeRefused
		}
		return dnsserver.NewResponse(res), dnsserver.HintSend, nil
	})
}

func startWire(t *testing.T, snap *snapshot.Snapshot) *dnsserver.Server {
	t.Helper()
	s, err := dnsserver.New(dnsserver.Config{
		UDPAddr: "127.0.0.1:0",
		TCPAddr: "127.0.0.1:0",
		Handler: handler(snap),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return s
}

func packQuery(t *testing.T, name string, typ model.RRType, id uint16, rd, cd bool) []byte {
	t.Helper()
	raw, err := dnswire.PackQuery(id, model.Query{
		Name:  model.Name(name),
		Type:  typ,
		Class: model.ClassIN,
		RD:    rd,
		CD:    cd,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
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
	buf := make([]byte, 8192)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf[:n]
}

func exchangeTCP(t *testing.T, addr net.Addr, payload []byte) []byte {
	t.Helper()
	c, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	if _, err := c.Write(hdr[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		t.Fatal(err)
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	body := make([]byte, n)
	if _, err := io.ReadFull(c, body); err != nil {
		t.Fatal(err)
	}
	return body
}

func headerFlags(msg []byte) (aa, rd, ra, ad, cd bool, rcode byte) {
	if len(msg) < dnswire.HeaderLen {
		return
	}
	aa = msg[2]&0x04 != 0
	rd = msg[2]&0x01 != 0
	ra = msg[3]&0x80 != 0
	ad = msg[3]&0x20 != 0
	cd = msg[3]&0x10 != 0
	rcode = msg[3] & 0x0F
	return
}

func containsIPv4(msg []byte, a, b, c, d byte) bool {
	pat := []byte{a, b, c, d}
	for i := 0; i+4 <= len(msg); i++ {
		if msg[i] == pat[0] && msg[i+1] == pat[1] && msg[i+2] == pat[2] && msg[i+3] == pat[3] {
			return true
		}
	}
	return false
}

func wireSnap(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	return snapOf(t, []model.Zone{
		authZone(
			rec("a", "ns1", model.TypeA, 30*time.Second, "10.42.0.53"),
			rec("w", "*.tools", model.TypeA, 30*time.Second, "10.42.0.20"),
		),
		overlayZone(
			rec("o", "special-api", model.TypeA, 30*time.Second, "10.42.0.30"),
		),
	}, 0)
}

func TestUDPTCPExactAndWildcard(t *testing.T) {
	s := startWire(t, wireSnap(t))

	udp := exchangeUDP(t, s.UDPAddr(), packQuery(t, "ns1.lab.example.net.", model.TypeA, 1, true, false))
	aa, _, ra, ad, cd, rcode := headerFlags(udp)
	if rcode != 0 || !aa || ra || ad || cd {
		t.Fatalf("udp exact flags aa=%v ra=%v ad=%v cd=%v rcode=%d", aa, ra, ad, cd, rcode)
	}
	if !containsIPv4(udp, 10, 42, 0, 53) {
		t.Fatal("udp exact missing A rdata")
	}

	tcp := exchangeTCP(t, s.TCPAddr(), packQuery(t, "alpha.tools.lab.example.net.", model.TypeA, 2, true, false))
	aa, _, ra, ad, cd, rcode = headerFlags(tcp)
	if rcode != 0 || !aa || ra || ad || cd {
		t.Fatalf("tcp wildcard flags aa=%v ra=%v ad=%v cd=%v rcode=%d", aa, ra, ad, cd, rcode)
	}
	if !containsIPv4(tcp, 10, 42, 0, 20) {
		t.Fatal("tcp wildcard missing A rdata")
	}
}

func TestUDPTCPFlagEquivalence(t *testing.T) {
	s := startWire(t, wireSnap(t))
	q := packQuery(t, "ns1.lab.example.net.", model.TypeA, 9, true, true)
	u := exchangeUDP(t, s.UDPAddr(), q)
	c := exchangeTCP(t, s.TCPAddr(), q)
	uaa, urd, ura, uad, ucd, urc := headerFlags(u)
	caa, crd, cra, cad, ccd, crc := headerFlags(c)
	if uaa != caa || urd != crd || ura != cra || uad != cad || ucd != ccd || urc != crc {
		t.Fatalf("udp aa=%v rd=%v ra=%v ad=%v cd=%v r=%d tcp aa=%v rd=%v ra=%v ad=%v cd=%v r=%d",
			uaa, urd, ura, uad, ucd, urc, caa, crd, cra, cad, ccd, crc)
	}
	if uad || cad || ucd || ccd || ura || cra {
		t.Fatal("AD/CD/RA must stay clear on local answers")
	}
	if !urd || !crd {
		t.Fatal("RD should be echoed by the transport")
	}
}

func TestUDPOverlayMissIsRefusedOnWire(t *testing.T) {
	s := startWire(t, wireSnap(t))
	out := exchangeUDP(t, s.UDPAddr(), packQuery(t, "other.vendor.example.", model.TypeA, 3, true, false))
	_, _, _, ad, _, rcode := headerFlags(out)
	if rcode != 5 { // REFUSED
		t.Fatalf("overlay miss rcode=%d, want REFUSED", rcode)
	}
	if ad {
		t.Fatal("forged AD")
	}
}

func TestUDPNXDOMAIN(t *testing.T) {
	s := startWire(t, wireSnap(t))
	out := exchangeUDP(t, s.UDPAddr(), packQuery(t, "nope.lab.example.net.", model.TypeA, 4, false, false))
	aa, _, _, ad, cd, rcode := headerFlags(out)
	if rcode != 3 { // NXDOMAIN
		t.Fatalf("rcode=%d", rcode)
	}
	if !aa || ad || cd {
		t.Fatalf("aa=%v ad=%v cd=%v", aa, ad, cd)
	}
}
