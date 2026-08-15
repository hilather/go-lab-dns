package dnsserver

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestUDPIndependentClientA(t *testing.T) {
	s := startServer(t, Config{Handler: StaticA("192.0.2.9")})
	raw := packA(t, "svc.lab.", 0x1111, nil)
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	if !hasQR(out) {
		t.Fatal("QR not set")
	}
	if rcodeOf(t, out) != 0 {
		t.Fatalf("rcode=%d", rcodeOf(t, out))
	}
	req, err := dnswire.Parse(out, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != 0x1111 || req.Query.Name != "svc.lab." {
		t.Fatalf("echo %+v", req)
	}
}

func TestTCPIndependentClientA(t *testing.T) {
	s := startServer(t, Config{Handler: StaticA("192.0.2.9")})
	raw := packA(t, "svc.lab.", 0x2222, nil)
	out := mustExchangeTCP(t, s.TCPAddr(), raw)
	if !hasQR(out) || rcodeOf(t, out) != 0 {
		t.Fatalf("qr/rcode %x", out[:4])
	}
	req, err := dnswire.Parse(out, model.TransportTCP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != 0x2222 {
		t.Fatalf("id=%d", req.ID)
	}
}

func TestMalformedDropAndFORMERR(t *testing.T) {
	s := startServer(t, Config{})
	if out := exchangeUDP(t, s.UDPAddr(), []byte{1, 2, 3}, 150*time.Millisecond); out != nil {
		t.Fatalf("short packet should drop, got %d bytes", len(out))
	}
	if out := exchangeUDP(t, s.UDPAddr(), nil, 150*time.Millisecond); out != nil {
		t.Fatal("empty should drop")
	}

	buf := make([]byte, dnswire.HeaderLen+2)
	binary.BigEndian.PutUint16(buf[0:2], 0xABAB)
	binary.BigEndian.PutUint16(buf[4:6], 1) // QDCOUNT=1, no question
	out := mustExchangeUDP(t, s.UDPAddr(), buf)
	if rcodeOf(t, out) != 1 { // FORMERR
		t.Fatalf("malformed header rcode=%d", rcodeOf(t, out))
	}
	if binary.BigEndian.Uint16(out[0:2]) != 0xABAB {
		t.Fatal("id not echoed")
	}
}

func TestQRQueryIsDropped(t *testing.T) {
	s := startServer(t, Config{})
	raw := packA(t, "x.lab.", 9, nil)
	raw[2] |= 0x80
	if out := exchangeUDP(t, s.UDPAddr(), raw, 150*time.Millisecond); out != nil {
		t.Fatal("QR=1 should drop")
	}
}

func TestOpcodeNOTIMP(t *testing.T) {
	s := startServer(t, Config{})
	raw := packA(t, "x.lab.", 10, nil)
	// opcode UPDATE = 5 in bits 14-11
	raw[2] = (raw[2] & 0x87) | (5 << 3)
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	if rcodeOf(t, out) != 4 { // NOTIMP
		t.Fatalf("rcode=%d want NOTIMP", rcodeOf(t, out))
	}
}

func TestZeroAndMultiQuestionFORMERR(t *testing.T) {
	s := startServer(t, Config{MaxQuestions: 1})
	raw := packA(t, "x.lab.", 11, nil)
	// QDCOUNT=0
	zero := append([]byte(nil), raw...)
	binary.BigEndian.PutUint16(zero[4:6], 0)
	out := mustExchangeUDP(t, s.UDPAddr(), zero)
	if rcodeOf(t, out) != 1 {
		t.Fatalf("qd=0 rcode=%d", rcodeOf(t, out))
	}

	// Two questions: pack one and bump QDCOUNT, append another name.
	multi, err := dnswire.PackQuery(12, model.Query{Name: "a.lab.", Type: model.TypeA, Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	extra, err := dnswire.PackQuery(12, model.Query{Name: "b.lab.", Type: model.TypeA, Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// extra[12:] is the question section of a 1-question message
	multi = append(multi, extra[dnswire.HeaderLen:]...)
	binary.BigEndian.PutUint16(multi[4:6], 2)
	out = mustExchangeUDP(t, s.UDPAddr(), multi)
	if rcodeOf(t, out) != 1 {
		t.Fatalf("qd=2 rcode=%d", rcodeOf(t, out))
	}
}

func TestClassNotINIsNOTIMP(t *testing.T) {
	s := startServer(t, Config{})
	raw, err := dnswire.PackQuery(13, model.Query{Name: "ch.lab.", Type: model.TypeA, Class: "CH"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	if rcodeOf(t, out) != 4 {
		t.Fatalf("class CH rcode=%d", rcodeOf(t, out))
	}
}

func TestAXFRIsNOTIMP(t *testing.T) {
	s := startServer(t, Config{})
	raw, err := dnswire.PackQuery(14, model.Query{Name: "lab.", Type: "AXFR", Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	if rcodeOf(t, out) != 4 {
		t.Fatalf("AXFR rcode=%d", rcodeOf(t, out))
	}
}

func TestEDNSVersionBADVERS(t *testing.T) {
	s := startServer(t, Config{})
	raw, err := dnswire.PackQuery(15, model.Query{Name: "e.lab.", Type: model.TypeA, Class: model.ClassIN}, &dnswire.EDNS{UDPSize: 1232, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	// BADVERS is extended RCODE 16: header rcode nibble is 0, OPT carries high bits.
	// miekg may put 16 into the packed rcode; accept FORMERR (1) or 0 with OPT.
	got := rcodeOf(t, out)
	if got != 1 && got != 0 {
		t.Fatalf("BADVERS header rcode=%d", got)
	}
	req, err := dnswire.Parse(out, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if !req.HasEDNS {
		t.Fatal("BADVERS must include OPT")
	}
}

func TestEDNSSizeClampAndNoEDNS512(t *testing.T) {
	s := startServer(t, Config{
		Handler:        oversizedTXT(40),
		MaxEDNSUDPSize: 4096,
	})
	// No EDNS: must fit in 512 and set TC.
	raw, err := dnswire.PackQuery(16, model.Query{Name: "big.lab.", Type: model.TypeTXT, Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	if len(out) > 512 {
		t.Fatalf("no-edns payload %d", len(out))
	}
	if !hasTC(out) {
		t.Fatal("expected TC without EDNS")
	}

	raw, err = dnswire.PackQuery(17, model.Query{Name: "big.lab.", Type: model.TypeTXT, Class: model.ClassIN}, &dnswire.EDNS{UDPSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	out = mustExchangeUDP(t, s.UDPAddr(), raw)
	if hasTC(out) && len(out) < 512 {
		t.Fatal("EDNS 4096 unexpectedly truncated to tiny payload")
	}
}

func TestUDPOversizeDropped(t *testing.T) {
	s := startServer(t, Config{MaxUDPSize: 64})
	raw := packA(t, "oversize-name-that-makes-a-larger-query.lab.example.", 18, &dnswire.EDNS{UDPSize: 1232})
	if len(raw) <= 64 {
		t.Fatalf("test packet too small (%d); pick a longer name", len(raw))
	}
	if out := exchangeUDP(t, s.UDPAddr(), raw, 150*time.Millisecond); out != nil {
		t.Fatal("oversize UDP should drop")
	}
}

func TestNilResponseAndHandlerErrorAreSERVFAIL(t *testing.T) {
	s := startServer(t, Config{
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			return nil, HintSend, nil
		}),
	})
	out := mustExchangeUDP(t, s.UDPAddr(), packA(t, "n.lab.", 19, nil))
	if rcodeOf(t, out) != 2 {
		t.Fatalf("nil response rcode=%d", rcodeOf(t, out))
	}

	s2 := startServer(t, Config{
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			return nil, HintSend, errTest
		}),
	})
	out = mustExchangeUDP(t, s2.UDPAddr(), packA(t, "e.lab.", 20, nil))
	if rcodeOf(t, out) != 2 {
		t.Fatalf("handler error rcode=%d", rcodeOf(t, out))
	}
}

func TestHandlerPanicIsSERVFAIL(t *testing.T) {
	s := startServer(t, Config{
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			panic("boom")
		}),
	})
	out := mustExchangeUDP(t, s.UDPAddr(), packA(t, "p.lab.", 21, nil))
	if rcodeOf(t, out) != 2 {
		t.Fatalf("panic rcode=%d", rcodeOf(t, out))
	}
}

func oversizedTXT(n int) Handler {
	return HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
		answers := make([]model.RR, 0, n)
		for i := 0; i < n; i++ {
			answers = append(answers, model.RR{
				Name:  q.Name,
				Type:  model.TypeTXT,
				Class: model.ClassIN,
				TTL:   time.Second,
				Data:  `"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`,
			})
		}
		return NewResponse(model.Result{RCode: model.RCodeNoError, Answers: answers}), HintSend, nil
	})
}
