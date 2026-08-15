package dnswire

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestUnpackUpstreamRoundTrip(t *testing.T) {
	req := &Request{
		HeaderOK: true,
		ID:       42,
		Query:    model.Query{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, RD: true, CD: true},
	}
	res := model.Result{
		RCode: model.RCodeNoError,
		CD:    true,
		Answers: []model.RR{{
			Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: 30 * time.Second, Data: "192.0.2.1",
		}},
	}
	raw, err := Encode(req, res, EncodeOpts{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnpackUpstream(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 42 || !got.QR || !got.CD {
		t.Fatalf("%+v", got)
	}
	if got.RCode != model.RCodeNoError || len(got.Answers) != 1 || got.Answers[0].Data != "192.0.2.1" {
		t.Fatalf("answers %+v", got.Answers)
	}
}

func TestUnpackUpstreamMalformed(t *testing.T) {
	if _, err := UnpackUpstream(nil); err != ErrEmpty {
		t.Fatalf("empty: %v", err)
	}
	if _, err := UnpackUpstream([]byte{1, 2, 3}); err != ErrShortHeader {
		t.Fatalf("short: %v", err)
	}
}
