package chaos

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// two64 is 2^64 as a float64. Uniform mapping is float64(u) / 2^64, never
// integer division.
var two64 = math.Ldexp(1, 64)

// EncodeHashV1 concatenates magic + ten length-prefixed fields.
// Each field is uint32 big-endian length + exactly that many bytes.
func EncodeHashV1(f HashFields) []byte {
	// 15-byte magic + 10 * 4 length prefixes + payloads
	n := len(Magic) + 40 +
		len(AlgorithmID) + len(f.Seed) + len(f.Revision) + len(f.PolicyID) +
		len(f.QNAME) + len(qtypeField(f.QTYPE)) + len(f.ClientGroup) +
		len(transportField(f.Transport)) + len(f.TimeBucket) + len(f.Nonce)
	buf := make([]byte, 0, n)
	buf = append(buf, Magic...)
	buf = appendField(buf, AlgorithmID)
	buf = appendField(buf, f.Seed)
	buf = appendField(buf, string(f.Revision))
	buf = appendField(buf, string(f.PolicyID))
	buf = appendField(buf, string(f.QNAME))
	buf = appendField(buf, qtypeField(f.QTYPE))
	buf = appendField(buf, f.ClientGroup)
	buf = appendField(buf, transportField(f.Transport))
	buf = appendField(buf, f.TimeBucket)
	buf = appendField(buf, f.Nonce)
	return buf
}

func appendField(buf []byte, s string) []byte {
	var lenb [4]byte
	binary.BigEndian.PutUint32(lenb[:], uint32(len(s)))
	buf = append(buf, lenb[:]...)
	return append(buf, s...)
}

func qtypeField(t model.RRType) string {
	return strings.ToUpper(string(t))
}

func transportField(t model.Transport) string {
	switch t {
	case model.TransportUDP:
		return "udp"
	case model.TransportTCP:
		return "tcp"
	default:
		return string(t)
	}
}

// HashV1 returns SHA-256(encoding) and the u0/u1 uniforms.
func HashV1(f HashFields) HashResult {
	sum := sha256.Sum256(EncodeHashV1(f))
	u0 := binary.BigEndian.Uint64(sum[0:8])
	u1 := binary.BigEndian.Uint64(sum[8:16])
	return HashResult{
		DigestHex: hex.EncodeToString(sum[:]),
		U0:        u0,
		U1:        u1,
		P:         float64(u0) / two64,
		W:         float64(u1) / two64,
	}
}

// TimeBucketString is hash-v1 field 9. Unset/zero bucket → empty.
// Otherwise floor(wall_UTC / bucket) * bucket as RFC3339 seconds Z.
// Truncation is toward −∞ on the Unix timeline.
func TimeBucketString(now time.Time, bucket time.Duration) string {
	if bucket <= 0 {
		return ""
	}
	sec := now.UTC().Unix()
	bsec := int64(bucket / time.Second)
	if bsec < 1 {
		// CFG rejects sub-second buckets; still fail-closed here.
		bsec = 1
	}
	floored := floorDiv(sec, bsec) * bsec
	return time.Unix(floored, 0).UTC().Format("2006-01-02T15:04:05Z")
}

func floorDiv(n, d int64) int64 {
	q := n / d
	if n < 0 && n%d != 0 {
		q--
	}
	return q
}

// UniformDelay maps u1/2^64 into [min, max).
func UniformDelay(min, max time.Duration, u1 uint64) time.Duration {
	if max <= min {
		return min
	}
	unit := float64(u1) / two64
	return min + time.Duration(unit*float64(max-min))
}

// DelayNonce is field 10 of the second hash-v1 encoding used for uniform
// delay: the literal "delay" concatenated with the original nonce.
func DelayNonce(nonce string) string {
	return "delay" + nonce
}

// PickOutcome walks configured order and selects the first outcome whose
// cumulative positive weight is > w*total. Weight ≤ 0 is ignored.
func PickOutcome(outcomes []model.ChaosOutcome, w float64) (model.ChaosOutcome, bool) {
	var total float64
	for _, o := range outcomes {
		if o.Weight > 0 {
			total += o.Weight
		}
	}
	if total == 0 || math.IsNaN(total) {
		return model.ChaosOutcome{}, false
	}
	t := w * total
	var cum float64
	var last model.ChaosOutcome
	var have bool
	for _, o := range outcomes {
		if o.Weight <= 0 {
			continue
		}
		cum += o.Weight
		last = o
		have = true
		if cum > t {
			return o, true
		}
	}
	// w ∈ [0,1) so t < total; keep last as a float-edge guard.
	return last, have
}

// ClientGroupField is hash-v1 field 7.
func ClientGroupField(sel model.ChaosSelector, group model.ClientGroupID, client string) string {
	if sel.SamplingKey == SamplingClientBucket {
		if client == "" {
			return ""
		}
		sum := sha256.Sum256([]byte(client))
		return hex.EncodeToString(sum[:8])
	}
	return string(group)
}
