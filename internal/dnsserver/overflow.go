package dnsserver

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

// DNS header flag bits (RFC 1035). Opcode is bits 14–11.
const (
	flagQR     = 1 << 15
	flagOpcode = 0x7800
	flagRD     = 1 << 8
)

// maxDNSName is the RFC 1035 wire limit for a domain name, including
// label-length octets and the root terminator.
const maxDNSName = 255

// overflowLimiter is one global token bucket for UDP inflight-overflow
// SERVFAIL replies. The admitted path does not take its mutex.
type overflowLimiter struct {
	disabled bool
	rate     float64
	burst    float64
	now      func() time.Time

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newOverflowLimiter(rate, burst float64, now func() time.Time) *overflowLimiter {
	if now == nil {
		now = time.Now
	}
	if rate < 0 {
		return &overflowLimiter{disabled: true, now: now}
	}
	if burst < 1 {
		burst = 1
	}
	return &overflowLimiter{
		rate:   rate,
		burst:  burst,
		now:    now,
		tokens: burst,
		last:   now(),
	}
}

func (l *overflowLimiter) allow() bool {
	if l == nil || l.disabled {
		return false
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.last = now
	} else if elapsed < 0 {
		// A stepped-back clock must not mint tokens.
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// overflowQuestionEnd reports the index just past the single question.
// ok is false when the datagram must be dropped: short header, QR=1,
// QDCOUNT other than 1, a compression pointer or extended label, a name
// that does not fit in 255 octets, or a truncated QTYPE/QCLASS.
// The walk does not follow compression pointers and does not read past
// the question.
func overflowQuestionEnd(pkt []byte) (end int, ok bool) {
	if len(pkt) < dnswire.HeaderLen {
		return 0, false
	}
	if pkt[2]&0x80 != 0 { // QR
		return 0, false
	}
	if binary.BigEndian.Uint16(pkt[4:6]) != 1 {
		return 0, false
	}
	i := dnswire.HeaderLen
	consumed := 0
	for {
		if i >= len(pkt) {
			return 0, false
		}
		lab := int(pkt[i])
		if lab == 0 {
			consumed++
			if consumed > maxDNSName {
				return 0, false
			}
			i++
			break
		}
		// Compression (11) and extended labels (01) are rejected.
		// The cheap path does not chase pointers.
		if lab&0xC0 != 0 {
			return 0, false
		}
		if consumed+1+lab > maxDNSName || i+1+lab > len(pkt) {
			return 0, false
		}
		consumed += 1 + lab
		i += 1 + lab
	}
	if i+4 > len(pkt) {
		return 0, false
	}
	return i + 4, true
}

// buildOverflowServfail copies the header and question and rewrites the
// header into a SERVFAIL response. end is from overflowQuestionEnd.
// The reply is header plus question only: answer, authority, and
// additional (including EDNS OPT) are omitted, so len(reply) <= len(pkt)
// whenever end <= len(pkt).
//
// RA is 0. This path does not classify the client. EncodeError does the
// same; the handler sets RA only for a known-forward client.
func buildOverflowServfail(pkt []byte, end int) []byte {
	reply := make([]byte, end)
	copy(reply, pkt[:end])
	flags := binary.BigEndian.Uint16(pkt[2:4])
	// model.RCode is a mnemonic, not the wire nibble. SERVFAIL is 2.
	out := uint16(flagQR) | (flags & flagOpcode) | (flags & flagRD) | 2
	binary.BigEndian.PutUint16(reply[2:4], out)
	binary.BigEndian.PutUint16(reply[4:6], 1)
	binary.BigEndian.PutUint16(reply[6:8], 0)
	binary.BigEndian.PutUint16(reply[8:10], 0)
	binary.BigEndian.PutUint16(reply[10:12], 0)
	return reply
}

// replyInflightOverflow answers one UDP datagram that could not take a
// slot. It runs on the read goroutine: no inflight slot, no extra
// goroutine, no dnswire.Parse. Malformed questions are dropped without
// spending a token.
func (s *Server) replyInflightOverflow(pkt []byte, addr net.Addr) {
	end, ok := overflowQuestionEnd(pkt)
	if !ok || !s.overflow.allow() {
		s.cfg.Metrics.IncAdmission("inflight", "drop")
		return
	}
	reply := buildOverflowServfail(pkt, end)
	s.cfg.Metrics.IncAdmission("inflight", string(model.RCodeServFail))
	_, _ = s.udp.WriteTo(reply, addr)
}
