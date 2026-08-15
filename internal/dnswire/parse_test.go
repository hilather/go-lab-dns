package dnswire

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

var loopback = netip.MustParseAddr("127.0.0.1")

func TestParsePackRoundTrip(t *testing.T) {
	q := model.Query{
		Name:      "example.com.",
		Type:      model.TypeA,
		Class:     model.ClassIN,
		RD:        true,
		Transport: model.TransportUDP,
	}
	raw, err := PackQuery(0x1234, q, &EDNS{UDPSize: 1232, DO: true})
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if !req.HeaderOK || req.ID != 0x1234 {
		t.Fatalf("header id=%d ok=%v", req.ID, req.HeaderOK)
	}
	if req.Query.Name != "example.com." {
		t.Fatalf("name=%q", req.Query.Name)
	}
	if req.Query.Type != model.TypeA || req.Query.Class != model.ClassIN {
		t.Fatalf("type=%s class=%s", req.Query.Type, req.Query.Class)
	}
	if !req.Query.RD || !req.HasEDNS || req.EDNS.UDPSize != 1232 || !req.EDNS.DO {
		t.Fatalf("rd/edns mismatch: %+v", req)
	}
	if req.Query.Client != loopback || req.Query.Transport != model.TransportUDP {
		t.Fatalf("client/transport %+v", req.Query)
	}
}

func TestParseCanonicalizesName(t *testing.T) {
	raw, err := PackQuery(1, model.Query{Name: "ExAmPle.COM", Type: model.TypeAAAA}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(raw, model.TransportTCP, netip.MustParseAddr("::1"))
	if err != nil {
		t.Fatal(err)
	}
	if req.Query.Name != "example.com." {
		t.Fatalf("name=%q", req.Query.Name)
	}
}

func TestParseEmptyAndShort(t *testing.T) {
	if _, err := Parse(nil, model.TransportUDP, loopback); !IsMalformed(err) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := Parse([]byte{1, 2, 3}, model.TransportUDP, loopback); !IsMalformed(err) {
		t.Fatalf("short: %v", err)
	}
}

func TestParseMalformedWithHeaderAllowsFORMERR(t *testing.T) {
	buf := make([]byte, HeaderLen+4)
	binary.BigEndian.PutUint16(buf[0:2], 0xBEEF)
	// QDCOUNT=1 but no question bytes that unpack
	binary.BigEndian.PutUint16(buf[4:6], 1)
	req, err := Parse(buf, model.TransportUDP, loopback)
	if err == nil {
		t.Fatal("expected malformed")
	}
	if req == nil || !req.HeaderOK || req.ID != 0xBEEF {
		t.Fatalf("want recoverable header, got %+v err=%v", req, err)
	}
}

func TestParseDoesNotPanicOnJunk(t *testing.T) {
	junk := [][]byte{
		{},
		{0},
		bytes.Repeat([]byte{0xff}, 64),
		bytes.Repeat([]byte{0x00}, 512),
		append([]byte{0, 1, 0x80, 0, 0, 1}, bytes.Repeat([]byte{0xc0, 0x00}, 40)...), // compression loop-ish
	}
	for i, j := range junk {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("case %d panicked: %v", i, rec)
				}
			}()
			_, _ = Parse(j, model.TransportUDP, loopback)
		}()
	}
}

func TestParseGenericType(t *testing.T) {
	raw, err := PackQuery(9, model.Query{Name: "x.example.", Type: "TYPE99", Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(req.Query.Type), "TYPE") && req.Query.Type != "SPF" {
		// TYPE99 is SPF in some tables; either form is a model type string.
		if req.Query.Type != "TYPE99" && req.Query.Type != "SPF" {
			t.Fatalf("type=%q", req.Query.Type)
		}
	}
}

func TestParseQRBit(t *testing.T) {
	raw, err := PackQuery(1, model.Query{Name: "a.example.", Type: model.TypeA}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw[2] |= 0x80
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if !req.QR {
		t.Fatal("expected QR")
	}
}

func TestEffectiveUDPSize(t *testing.T) {
	if n := EffectiveUDPSize(&Request{}, 4096); n != MinUDPSize {
		t.Fatalf("no edns: %d", n)
	}
	if n := EffectiveUDPSize(&Request{HasEDNS: true, EDNS: EDNS{UDPSize: 100}}, 4096); n != MinUDPSize {
		t.Fatalf("tiny edns: %d", n)
	}
	if n := EffectiveUDPSize(&Request{HasEDNS: true, EDNS: EDNS{UDPSize: 8192}}, 4096); n != 4096 {
		t.Fatalf("clamp: %d", n)
	}
	if n := EffectiveUDPSize(&Request{HasEDNS: true, EDNS: EDNS{UDPSize: 1232}}, 4096); n != 1232 {
		t.Fatalf("pass: %d", n)
	}
}
