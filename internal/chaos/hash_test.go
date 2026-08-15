package chaos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

type hashVector struct {
	Name        string `json:"name"`
	Seed        string `json:"seed"`
	Revision    string `json:"revision"`
	PolicyID    string `json:"policyId"`
	QNAME       string `json:"qname"`
	QTYPE       string `json:"qtype"`
	ClientGroup string `json:"clientGroup"`
	Transport   string `json:"transport"`
	TimeBucket  string `json:"timeBucket"`
	Now         string `json:"now"`
	Nonce       string `json:"nonce"`
	Field9      string `json:"field9"`
	DigestHex   string `json:"digestHex"`
	U0          string `json:"u0"`
	U1          string `json:"u1"`
}

func testFields(v hashVector) (HashFields, time.Time, time.Duration) {
	now, err := time.Parse(time.RFC3339Nano, v.Now)
	if err != nil {
		now, err = time.Parse(time.RFC3339, v.Now)
		if err != nil {
			panic(err)
		}
	}
	var bucket time.Duration
	if v.TimeBucket != "" {
		bucket, err = time.ParseDuration(v.TimeBucket)
		if err != nil {
			panic(err)
		}
	}
	return HashFields{
		Seed:        v.Seed,
		Revision:    model.Revision(v.Revision),
		PolicyID:    model.PolicyID(v.PolicyID),
		QNAME:       model.Name(v.QNAME),
		QTYPE:       model.RRType(v.QTYPE),
		ClientGroup: v.ClientGroup,
		Transport:   model.Transport(v.Transport),
		TimeBucket:  TimeBucketString(now, bucket),
		Nonce:       v.Nonce,
	}, now, bucket
}

func TestHashV1TimeBucketSameSecondAndNext(t *testing.T) {
	vectors := loadVectors(t)
	if len(vectors) < 3 {
		t.Fatalf("need at least 3 goldens, got %d", len(vectors))
	}
	var sameA, sameB, next hashVector
	for _, v := range vectors {
		switch v.Name {
		case "same-second-a":
			sameA = v
		case "same-second-b":
			sameB = v
		case "next-second":
			next = v
		}
	}
	if sameA.Name == "" || sameB.Name == "" || next.Name == "" {
		t.Fatal("missing named timeBucket goldens")
	}
	if sameA.Field9 != sameB.Field9 {
		t.Fatalf("same UTC second must share field 9: %s vs %s", sameA.Field9, sameB.Field9)
	}
	if sameA.DigestHex != sameB.DigestHex {
		t.Fatalf("same UTC second must share digest: %s vs %s", sameA.DigestHex, sameB.DigestHex)
	}
	if next.Field9 == sameA.Field9 {
		t.Fatal("next second must change field 9")
	}
	if next.DigestHex == sameA.DigestHex {
		t.Fatal("next second must change digest")
	}
	for _, v := range []hashVector{sameA, sameB, next} {
		f, _, _ := testFields(v)
		if f.TimeBucket != v.Field9 {
			t.Fatalf("%s field9=%s want %s", v.Name, f.TimeBucket, v.Field9)
		}
		got := HashV1(f)
		if got.DigestHex != v.DigestHex {
			t.Fatalf("%s digest=%s want %s (GOARCH=%s)", v.Name, got.DigestHex, v.DigestHex, runtime.GOARCH)
		}
		if FormatU64(got.U0) != v.U0 || FormatU64(got.U1) != v.U1 {
			t.Fatalf("%s u0/u1=%s/%s want %s/%s", v.Name, FormatU64(got.U0), FormatU64(got.U1), v.U0, v.U1)
		}
	}
}

func TestHashV1BitForBitGoldens(t *testing.T) {
	for _, v := range loadVectors(t) {
		f, _, _ := testFields(v)
		got := HashV1(f)
		if got.DigestHex != v.DigestHex {
			t.Errorf("%s digest=%s want %s", v.Name, got.DigestHex, v.DigestHex)
		}
	}
}

func TestHashV1StableAcrossRestarts(t *testing.T) {
	f := HashFields{
		Seed: "startup-v1", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PolicyID: "slow-tools", QNAME: "foo.tools.lab.example.net.", QTYPE: "A",
		ClientGroup: "test-devices", Transport: "udp",
		TimeBucket: "2026-08-15T20:00:00Z",
	}
	a := HashV1(f)
	b := HashV1(f)
	if a != b {
		t.Fatalf("same inputs diverged: %+v vs %+v", a, b)
	}
}

func TestHashV1SeedAndPolicyDiverge(t *testing.T) {
	base := HashFields{
		Seed: "s1", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		PolicyID: "p1", QNAME: "a.example.", QTYPE: "A",
		ClientGroup: "g", Transport: "udp",
	}
	h1 := HashV1(base)
	base.Seed = "s2"
	h2 := HashV1(base)
	base.Seed = "s1"
	base.PolicyID = "p2"
	h3 := HashV1(base)
	if h1.DigestHex == h2.DigestHex || h1.DigestHex == h3.DigestHex || h2.DigestHex == h3.DigestHex {
		t.Fatal("seed/policy changes must diverge")
	}
}

func TestTimeBucketFloorNegative(t *testing.T) {
	// 1969-12-31T23:59:59.5Z → Unix -1 (toward −∞) → field 9 at that second.
	now := time.Date(1969, 12, 31, 23, 59, 59, 500000000, time.UTC)
	got := TimeBucketString(now, time.Second)
	if got != "1969-12-31T23:59:59Z" {
		t.Fatalf("got %s", got)
	}
}

func TestPickOutcomeOrder(t *testing.T) {
	outs := []model.ChaosOutcome{
		{ID: "a", Weight: 1},
		{ID: "b", Weight: 1},
		{ID: "skip", Weight: 0},
		{ID: "c", Weight: 2},
	}
	// total=4; t=0 → first with cum>0 is a; t=0.99*4=3.96 → c
	got, ok := PickOutcome(outs, 0)
	if !ok || got.ID != "a" {
		t.Fatalf("w=0 got %+v", got)
	}
	got, ok = PickOutcome(outs, 0.99)
	if !ok || got.ID != "c" {
		t.Fatalf("w=0.99 got %+v", got)
	}
	if _, ok := PickOutcome([]model.ChaosOutcome{{ID: "z", Weight: 0}}, 0.5); ok {
		t.Fatal("zero total must skip")
	}
}

func TestUniformDelayHalfOpen(t *testing.T) {
	d := UniformDelay(100*time.Millisecond, 200*time.Millisecond, 0)
	if d != 100*time.Millisecond {
		t.Fatalf("u1=0 → min, got %s", d)
	}
	d = UniformDelay(100*time.Millisecond, 100*time.Millisecond, ^uint64(0))
	if d != 100*time.Millisecond {
		t.Fatalf("min==max, got %s", d)
	}
}

func loadVectors(t *testing.T) []hashVector {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "hash-v1", "vectors.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []hashVector
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
