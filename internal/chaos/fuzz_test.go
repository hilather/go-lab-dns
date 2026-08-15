package chaos

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func FuzzHashV1(f *testing.F) {
	f.Add("seed", "sha256:aa", "pol", "a.example.", "A", "g", "udp", int64(1), "", int64(0))
	f.Add("", "", "", "", "", "", "", int64(0), "nonce", int64(-1))
	f.Fuzz(func(t *testing.T, seed, rev, pol, qname, qtype, group, transport string, bucketSec int64, nonce string, unix int64) {
		var bucket time.Duration
		if bucketSec > 0 && bucketSec < 86400*365 {
			bucket = time.Duration(bucketSec) * time.Second
		}
		now := time.Unix(unix, 0).UTC()
		_ = HashV1(HashFields{
			Seed:        seed,
			Revision:    model.Revision(rev),
			PolicyID:    model.PolicyID(pol),
			QNAME:       model.Name(qname),
			QTYPE:       model.RRType(qtype),
			ClientGroup: group,
			Transport:   model.Transport(transport),
			TimeBucket:  TimeBucketString(now, bucket),
			Nonce:       nonce,
		})
		_, _ = PickOutcome([]model.ChaosOutcome{
			{ID: "a", Weight: float64(len(seed))},
			{ID: "b", Weight: float64(len(nonce))},
		}, 0.3)
	})
}
