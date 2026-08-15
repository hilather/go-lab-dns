package dnswire

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func mustParse(t *testing.T, raw []byte) *Request {
	t.Helper()
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestEncodeEchoesIDQuestionAndSetsQR(t *testing.T) {
	raw, err := PackQuery(0x4242, model.Query{Name: "svc.lab.", Type: model.TypeA, RD: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := mustParse(t, raw)
	out, err := Encode(req, model.Result{
		RCode: model.RCodeNoError,
		AA:    true,
		Answers: []model.RR{{
			Name:  "svc.lab.",
			Type:  model.TypeA,
			Class: model.ClassIN,
			TTL:   30 * time.Second,
			Data:  "192.0.2.10",
		}},
	}, EncodeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(out, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 0x4242 || !got.QR {
		t.Fatalf("id=%d qr=%v", got.ID, got.QR)
	}
	if got.Query.Name != "svc.lab." || got.Query.Type != model.TypeA {
		t.Fatalf("question %+v", got.Query)
	}
}

func TestEncodeErrorFORMERRWithoutQuestion(t *testing.T) {
	req := &Request{HeaderOK: true, ID: 7}
	out, err := EncodeError(req, model.RCodeFormErr, EncodeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < HeaderLen {
		t.Fatalf("short reply %d", len(out))
	}
	// QR=1, RCODE=FORMERR (1)
	if out[2]&0x80 == 0 {
		t.Fatal("QR not set")
	}
	if out[3]&0x0F != 1 {
		t.Fatalf("rcode=%d", out[3]&0x0F)
	}
}

func TestEncodeUnknownRcodeIsSERVFAIL(t *testing.T) {
	raw, err := PackQuery(1, model.Query{Name: "x.", Type: model.TypeA}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := mustParse(t, raw)
	out, err := Encode(req, model.Result{RCode: model.RCode("NOPE")}, EncodeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if out[3]&0x0F != 2 { // SERVFAIL
		t.Fatalf("rcode=%d, want SERVFAIL", out[3]&0x0F)
	}
}

func TestEncodeForceTruncateSetsTC(t *testing.T) {
	raw, err := PackQuery(3, model.Query{Name: "big.lab.", Type: model.TypeTXT}, &EDNS{UDPSize: 1232})
	if err != nil {
		t.Fatal(err)
	}
	req := mustParse(t, raw)
	out, err := Encode(req, model.Result{
		RCode: model.RCodeNoError,
		Answers: []model.RR{{
			Name:  "big.lab.",
			Type:  model.TypeTXT,
			Class: model.ClassIN,
			TTL:   time.Second,
			Data:  `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		}},
	}, EncodeOpts{ForceTruncate: true, MaxUDPSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	if out[2]&0x02 == 0 {
		t.Fatal("TC not set")
	}
}

func TestEncodeRespectsUDPSize(t *testing.T) {
	raw, err := PackQuery(4, model.Query{Name: "t.lab.", Type: model.TypeTXT}, &EDNS{UDPSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	req := mustParse(t, raw)
	answers := make([]model.RR, 0, 20)
	for i := 0; i < 20; i++ {
		answers = append(answers, model.RR{
			Name:  "t.lab.",
			Type:  model.TypeTXT,
			Class: model.ClassIN,
			TTL:   time.Second,
			Data:  `"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`,
		})
	}
	out, err := Encode(req, model.Result{RCode: model.RCodeNoError, Answers: answers}, EncodeOpts{
		MaxUDPSize: EffectiveUDPSize(req, 4096),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > 512 {
		t.Fatalf("payload %d exceeds 512", len(out))
	}
	if out[2]&0x02 == 0 {
		t.Fatal("expected TC on oversized UDP")
	}
}

func TestEncodeNoHeader(t *testing.T) {
	if _, err := Encode(nil, model.Result{}, EncodeOpts{}); err != ErrNoHeader {
		t.Fatalf("nil req: %v", err)
	}
	if _, err := Encode(&Request{}, model.Result{}, EncodeOpts{}); err != ErrNoHeader {
		t.Fatalf("no header: %v", err)
	}
}

func TestEncodeBadVersIncludesOPT(t *testing.T) {
	raw, err := PackQuery(5, model.Query{Name: "e.lab.", Type: model.TypeA}, &EDNS{UDPSize: 1232, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if !req.HasEDNS || req.EDNS.Version != 1 {
		t.Fatalf("edns %+v", req.EDNS)
	}
	out, err := EncodeError(req, model.RCodeFormErr, EncodeOpts{BadVers: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(out, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasEDNS {
		t.Fatal("BADVERS reply missing OPT")
	}
	if got.HasEDNS && got.EDNS.Version != 0 {
		t.Fatalf("OPT version %d, want 0", got.EDNS.Version)
	}
	if out[3]&0x0F != 0 {
		t.Fatalf("BADVERS header rcode=%d, want 0", out[3]&0x0F)
	}
	if got.EDNS.ExtendedRcode != 16 {
		t.Fatalf("EXTENDED-RCODE=%d, want 16", got.EDNS.ExtendedRcode)
	}
}

func TestEncodeEDEOption(t *testing.T) {
	raw, err := PackQuery(9, model.Query{Name: "e.lab.", Type: model.TypeA}, &EDNS{UDPSize: 1232})
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(req, model.Result{
		RCode: model.RCodeServFail,
		EDE:   &model.EDE{Code: 0, Text: "lab-injected-failure"},
	}, EncodeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsEDEText(out, "lab-injected-failure") {
		t.Fatal("encoded response missing EDE text")
	}
}

func containsEDEText(raw []byte, text string) bool {
	need := []byte(text)
	for i := 0; i+len(need) <= len(raw); i++ {
		if string(raw[i:i+len(need)]) == text {
			return true
		}
	}
	return false
}

func TestEncodeFORMERROmitsUnparsedQuestion(t *testing.T) {
	req := &Request{HeaderOK: true, ID: 0xABAB, QDCount: 1}
	out, err := EncodeError(req, model.RCodeFormErr, EncodeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < HeaderLen {
		t.Fatal("short")
	}
	qd := uint16(out[4])<<8 | uint16(out[5])
	if qd != 0 {
		t.Fatalf("fabricated QDCOUNT=%d (must not echo . IN A)", qd)
	}
}

func TestPackQueryEDNSVersion(t *testing.T) {
	raw, err := PackQuery(1, model.Query{Name: "v.", Type: model.TypeA}, &EDNS{UDPSize: 512, Version: 3})
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(raw, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if req.EDNS.Version != 3 {
		t.Fatalf("version=%d", req.EDNS.Version)
	}
}
