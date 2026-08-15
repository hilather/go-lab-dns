package dnswire

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func FuzzParse(f *testing.F) {
	seeds := [][]byte{
		nil,
		{},
		{0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0},
	}
	q, err := PackQuery(0x1111, model.Query{Name: "fuzz.example.", Type: model.TypeA, RD: true}, nil)
	if err != nil {
		f.Fatal(err)
	}
	seeds = append(seeds, q)
	edns, err := PackQuery(0x2222, model.Query{Name: "edns.example.", Type: model.TypeAAAA, RD: true}, &EDNS{UDPSize: 1232})
	if err != nil {
		f.Fatal(err)
	}
	seeds = append(seeds, edns)
	for _, s := range seeds {
		f.Add(s)
	}
	// Seed files under testdata/fuzz/FuzzParse are also picked up by go test -fuzz.
	f.Fuzz(func(t *testing.T, data []byte) {
		req, err := Parse(data, model.TransportUDP, loopback)
		if req == nil && err == nil {
			t.Fatal("Parse returned nil, nil")
		}
		if req != nil && req.HeaderOK {
			_, _ = EncodeError(req, model.RCodeFormErr, EncodeOpts{})
			_, _ = Encode(req, model.Result{RCode: model.RCodeNXDomain}, EncodeOpts{
				MaxUDPSize:     EffectiveUDPSize(req, 4096),
				ForceTruncate:  len(data)%5 == 0,
				BadVers:        req.HasEDNS && req.EDNS.Version != 0,
				MaxEDNSUDPSize: 4096,
			})
		}
	})
}

func TestParseCorpusNoPanic(t *testing.T) {
	root := moduleRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "testdata", "packets", "*.raw"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("missing packet corpus under testdata/packets")
	}
	fuzzSeeds, err := filepath.Glob(filepath.Join("testdata", "fuzz", "FuzzParse", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fuzzSeeds) == 0 {
		t.Fatal("missing go-fuzz seeds under testdata/fuzz/FuzzParse")
	}
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("%s panicked: %v", p, rec)
				}
			}()
			_, _ = Parse(data, model.TransportUDP, loopback)
			_, _ = Parse(data, model.TransportTCP, loopback)
		}()
	}
}
