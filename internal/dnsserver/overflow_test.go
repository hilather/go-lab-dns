package dnsserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func blockingHandler(entered chan struct{}, release <-chan struct{}) Handler {
	var once sync.Once
	return HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		case <-ctx.Done():
		}
		return NewResponse(model.Result{RCode: model.RCodeNoError}), HintSend, nil
	})
}

// holdInflight starts a UDP server with one slot and blocks that slot in
// the handler. release unblocks it. The returned server is shut down with
// the test.
func holdInflight(t *testing.T, cfg Config) (s *Server, release func()) {
	t.Helper()
	entered := make(chan struct{})
	rel := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(rel) }) }
	t.Cleanup(release)

	cfg.UDPAddr = "127.0.0.1:0"
	cfg.TCPAddr = ""
	cfg.MaxInflight = 1
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 5 * time.Second
	}
	cfg.Handler = blockingHandler(entered, rel)
	s = startServer(t, cfg)

	raw := packA(t, "held.lab.", 1, nil)
	go func() {
		c, err := net.Dial("udp", s.UDPAddr().String())
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_ = c.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = c.Write(raw)
		buf := make([]byte, 512)
		_, _ = c.Read(buf)
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("holder did not acquire the inflight slot")
	}
	if n := len(s.inflight); n != 1 {
		t.Fatalf("inflight=%d after holder entered; want 1", n)
	}
	return s, release
}

func assertOverflowServfail(t *testing.T, query, reply []byte) {
	t.Helper()
	end, ok := overflowQuestionEnd(query)
	if !ok {
		t.Fatal("query is not one uncompressed question")
	}
	if len(reply) != end {
		t.Fatalf("reply len %d, question end %d", len(reply), end)
	}
	if len(reply) > len(query) {
		t.Fatalf("reply %d larger than query %d", len(reply), len(query))
	}
	if !bytes.Equal(reply[:2], query[:2]) {
		t.Fatal("id not copied")
	}
	if !bytes.Equal(reply[dnswire.HeaderLen:], query[dnswire.HeaderLen:end]) {
		t.Fatal("question not echoed")
	}
	qf := binary.BigEndian.Uint16(query[2:4])
	rf := binary.BigEndian.Uint16(reply[2:4])
	if rf&flagQR == 0 {
		t.Fatal("QR not set")
	}
	if rf&flagOpcode != qf&flagOpcode {
		t.Fatalf("opcode changed: query %04x reply %04x", qf, rf)
	}
	if rf&flagRD != qf&flagRD {
		t.Fatalf("RD changed: query %04x reply %04x", qf, rf)
	}
	if rf&0x000F != 2 {
		t.Fatalf("rcode=%d", rf&0x000F)
	}
	if extra := rf &^ (flagQR | flagOpcode | flagRD | 0x000F); extra != 0 {
		t.Fatalf("flags besides QR, opcode, RD, RCODE: %04x", rf)
	}
	if binary.BigEndian.Uint16(reply[4:6]) != 1 ||
		binary.BigEndian.Uint16(reply[6:8]) != 0 ||
		binary.BigEndian.Uint16(reply[8:10]) != 0 ||
		binary.BigEndian.Uint16(reply[10:12]) != 0 {
		t.Fatalf("counts qd=%d an=%d ns=%d ar=%d",
			binary.BigEndian.Uint16(reply[4:6]),
			binary.BigEndian.Uint16(reply[6:8]),
			binary.BigEndian.Uint16(reply[8:10]),
			binary.BigEndian.Uint16(reply[10:12]))
	}
}

func TestUDPInflightOverflowServfail(t *testing.T) {
	spy := &spyMetrics{}
	s, release := holdInflight(t, Config{Metrics: spy})
	raw := packA(t, "over.lab.", 0xBEEF, nil)
	out := mustExchangeUDP(t, s.UDPAddr(), raw)
	assertOverflowServfail(t, raw, out)
	req, err := dnswire.Parse(out, model.TransportUDP, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != 0xBEEF || req.Query.Name != "over.lab." || req.RA || !req.RD {
		t.Fatalf("parsed %+v", req)
	}
	if n := len(s.inflight); n != 1 {
		t.Fatalf("inflight=%d; overflow must not take or free the slot", n)
	}
	if !strings.Contains(spy.joined(), "admit|inflight|SERVFAIL") {
		t.Fatalf("metrics:\n%s", spy.joined())
	}
	release()
}

func TestUDPInflightOverflowRateDropsSecond(t *testing.T) {
	// Rate 1 and burst 1: the first overflow is answered, the second in
	// the same second is dropped. Rate 0 is the unset value and becomes
	// the 64/s default, so it would still answer the second packet.
	clk := testutil.NewFakeClock(time.Unix(1_700_000_000, 0))
	spy := &spyMetrics{}
	s, _ := holdInflight(t, Config{
		Metrics:               spy,
		Clock:                 clk,
		OverflowServfailRate:  1,
		OverflowServfailBurst: 1,
	})
	first := packA(t, "one.lab.", 0x1001, nil)
	out := mustExchangeUDP(t, s.UDPAddr(), first)
	assertOverflowServfail(t, first, out)

	second := packA(t, "two.lab.", 0x1002, nil)
	if got := exchangeUDP(t, s.UDPAddr(), second, 150*time.Millisecond); got != nil {
		t.Fatalf("second overflow answered (%d bytes); rate 1 burst 1 must drop it", len(got))
	}
	dump := spy.joined()
	if !strings.Contains(dump, "admit|inflight|SERVFAIL") || !strings.Contains(dump, "admit|inflight|drop") {
		t.Fatalf("metrics:\n%s", dump)
	}
}

func TestUDPInflightOverflowMalformedDropped(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_700_000_000, 0))
	spy := &spyMetrics{}
	s, _ := holdInflight(t, Config{
		Metrics:               spy,
		Clock:                 clk,
		OverflowServfailRate:  1,
		OverflowServfailBurst: 1,
	})
	valid := packA(t, "ok.lab.", 0x2222, nil)
	qr := append([]byte(nil), valid...)
	qr[2] |= 0x80
	qd := append([]byte(nil), valid...)
	binary.BigEndian.PutUint16(qd[4:6], 0)
	// www + compression pointer. The walker must not follow it.
	compressed := make([]byte, dnswire.HeaderLen+3+2+4)
	binary.BigEndian.PutUint16(compressed[0:2], 0x3333)
	compressed[2] = 0x01
	binary.BigEndian.PutUint16(compressed[4:6], 1)
	compressed[12] = 3
	copy(compressed[13:16], "www")
	compressed[16] = 0xC0
	compressed[17] = 12
	truncated := []byte{0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 3, 'a'}

	for _, pkt := range [][]byte{qr, qd, compressed, truncated, {1, 2, 3}} {
		if got := exchangeUDP(t, s.UDPAddr(), pkt, 150*time.Millisecond); got != nil {
			t.Fatalf("malformed overflow answered (%d bytes): %x", len(got), pkt)
		}
	}
	out := mustExchangeUDP(t, s.UDPAddr(), valid)
	assertOverflowServfail(t, valid, out)
	if n := strings.Count(spy.joined(), "admit|inflight|SERVFAIL"); n != 1 {
		t.Fatalf("overflow SERVFAIL count=%d, want 1 (malformed must not take the token):\n%s", n, spy.joined())
	}
}

func TestUDPInflightOverflowRespectsQueryRate(t *testing.T) {
	spy := &spyMetrics{}
	s, _ := holdInflight(t, Config{
		Metrics:         spy,
		QueryRatePerSec: 1,
		QueryBurst:      1,
	})
	raw := packA(t, "limited.lab.", 0x4444, nil)
	if got := exchangeUDP(t, s.UDPAddr(), raw, 150*time.Millisecond); got != nil {
		t.Fatalf("rate-limited overflow answered (%d bytes)", len(got))
	}
	dump := spy.joined()
	if !strings.Contains(dump, "admit|rate|") {
		t.Fatalf("missing rate drop:\n%s", dump)
	}
	if strings.Contains(dump, "admit|inflight|SERVFAIL") {
		t.Fatalf("rate limit ran after overflow SERVFAIL:\n%s", dump)
	}
}

func TestUDPInflightOverflowDisabledDrops(t *testing.T) {
	spy := &spyMetrics{}
	s, _ := holdInflight(t, Config{
		Metrics:              spy,
		OverflowServfailRate: -1,
	})
	raw := packA(t, "off.lab.", 0x5555, nil)
	if got := exchangeUDP(t, s.UDPAddr(), raw, 150*time.Millisecond); got != nil {
		t.Fatalf("disabled overflow answered (%d bytes)", len(got))
	}
	dump := spy.joined()
	if !strings.Contains(dump, "admit|inflight|drop") {
		t.Fatalf("metrics:\n%s", dump)
	}
	if strings.Contains(dump, "admit|inflight|SERVFAIL") {
		t.Fatalf("disabled rate still sent SERVFAIL:\n%s", dump)
	}
}

func TestBuildOverflowServfail(t *testing.T) {
	plain := packA(t, "svc.lab.", 0xABCD, nil)
	end, ok := overflowQuestionEnd(plain)
	if !ok {
		t.Fatal("plain walk")
	}
	reply := buildOverflowServfail(plain, end)
	if len(reply) > len(plain) {
		t.Fatalf("plain reply %d > query %d", len(reply), len(plain))
	}
	if len(reply) != len(plain) {
		t.Fatalf("plain reply %d, query %d; a question-only query is echoed in full", len(reply), len(plain))
	}
	assertOverflowServfail(t, plain, reply)

	edns := packA(t, "svc.lab.", 0xABCD, &dnswire.EDNS{UDPSize: 1232, DO: true})
	end, ok = overflowQuestionEnd(edns)
	if !ok {
		t.Fatal("edns walk")
	}
	reply = buildOverflowServfail(edns, end)
	if len(reply) > len(edns) {
		t.Fatalf("edns reply %d > query %d", len(reply), len(edns))
	}
	if len(reply) >= len(edns) {
		t.Fatal("OPT was not dropped")
	}
	assertOverflowServfail(t, edns, reply)

	noRD, err := dnswire.PackQuery(9, model.Query{Name: "nord.lab.", Type: model.TypeA, Class: model.ClassIN}, nil)
	if err != nil {
		t.Fatal(err)
	}
	end, ok = overflowQuestionEnd(noRD)
	if !ok {
		t.Fatal("nord walk")
	}
	reply = buildOverflowServfail(noRD, end)
	assertOverflowServfail(t, noRD, reply)
	if reply[2]&0x01 != 0 {
		t.Fatal("RD=0 was set")
	}

	dirty := append([]byte(nil), plain...)
	dirty[2] = (dirty[2] & 0x87) | (5 << 3) | 0x04 | 0x02 // opcode UPDATE, AA, TC
	dirty[3] = 0x80 | 0x20 | 0x10                         // RA, AD, CD
	end, ok = overflowQuestionEnd(dirty)
	if !ok {
		t.Fatal("dirty walk")
	}
	reply = buildOverflowServfail(dirty, end)
	assertOverflowServfail(t, dirty, reply)

	withTail := append(append([]byte(nil), plain...), 0xFF, 0xFF, 0xFF, 0xFF)
	end, ok = overflowQuestionEnd(withTail)
	if !ok || end != len(plain) {
		t.Fatalf("tail walk end=%d ok=%v", end, ok)
	}
	reply = buildOverflowServfail(withTail, end)
	if len(reply) > len(withTail) || len(reply) != len(plain) {
		t.Fatalf("tail reply %d", len(reply))
	}

	root := packetWithLabels(t, 0x10, nil)
	end, ok = overflowQuestionEnd(root)
	if !ok {
		t.Fatal("root rejected")
	}
	reply = buildOverflowServfail(root, end)
	if len(reply) > len(root) {
		t.Fatal("root grew")
	}
	assertOverflowServfail(t, root, reply)

	fits := packetWithLabels(t, 0x11, []int{63, 63, 63, 61})
	if _, ok := overflowQuestionEnd(fits); !ok {
		t.Fatal("255-octet name rejected")
	}
	reply = buildOverflowServfail(fits, mustEnd(t, fits))
	if len(reply) > len(fits) {
		t.Fatal("max name grew")
	}

	for _, labels := range [][]int{{63, 63, 63, 62}, {64}, {0xC0}} {
		pkt := packetWithLabels(t, 0x12, labels)
		if _, ok := overflowQuestionEnd(pkt); ok {
			t.Fatalf("accepted labels %v", labels)
		}
	}
	short := []byte{0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0}
	if _, ok := overflowQuestionEnd(short); ok {
		t.Fatal("short header accepted")
	}
	if _, ok := overflowQuestionEnd(nil); ok {
		t.Fatal("empty accepted")
	}
	qr := append([]byte(nil), plain...)
	qr[2] |= 0x80
	if _, ok := overflowQuestionEnd(qr); ok {
		t.Fatal("QR=1 accepted")
	}
	multi := append([]byte(nil), plain...)
	binary.BigEndian.PutUint16(multi[4:6], 2)
	if _, ok := overflowQuestionEnd(multi); ok {
		t.Fatal("QDCOUNT=2 accepted")
	}
}

func TestOverflowLimiter(t *testing.T) {
	cur := time.Unix(0, 0)
	l := newOverflowLimiter(1, 1, func() time.Time { return cur })
	if !l.allow() || l.allow() {
		t.Fatal("burst 1 allowed a second reply")
	}
	cur = cur.Add(time.Second)
	if !l.allow() {
		t.Fatal("rate 1 did not refill after one second")
	}
	if l.allow() {
		t.Fatal("refill granted more than one token")
	}
	cur = cur.Add(-time.Hour)
	if l.allow() {
		t.Fatal("clock step back minted a token")
	}

	// The raw bucket does not apply the config default. Rate 0 does not
	// refill, so the second take is dropped. Config value 0 becomes 64
	// in withDefaults before this constructor runs.
	zero := newOverflowLimiter(0, 1, func() time.Time { return cur })
	if !zero.allow() || zero.allow() {
		t.Fatal("rate 0 burst 1 should drop the second take")
	}
	cur = cur.Add(time.Hour)
	if zero.allow() {
		t.Fatal("rate 0 refilled")
	}

	if newOverflowLimiter(-1, 16, time.Now).allow() {
		t.Fatal("negative rate allowed a reply")
	}
	var none *overflowLimiter
	if none.allow() {
		t.Fatal("nil limiter allowed a reply")
	}
}

func TestNewRejectsOverflowConfig(t *testing.T) {
	cases := []Config{
		{OverflowServfailRate: math.NaN()},
		{OverflowServfailRate: math.Inf(1)},
		{OverflowServfailRate: math.Inf(-1)},
		{OverflowServfailBurst: math.NaN()},
		{OverflowServfailBurst: -1},
		{OverflowServfailBurst: 0.5},
	}
	for _, cfg := range cases {
		if _, err := New(cfg); err == nil {
			t.Fatalf("accepted %+v", cfg)
		}
	}
	s, err := New(Config{UDPAddr: "127.0.0.1:0", Handler: StaticA("192.0.2.1")})
	if err != nil {
		t.Fatal(err)
	}
	if s.cfg.OverflowServfailRate != DefaultOverflowServfailRate || s.cfg.OverflowServfailBurst != DefaultOverflowServfailBurst {
		t.Fatalf("defaults rate=%v burst=%v", s.cfg.OverflowServfailRate, s.cfg.OverflowServfailBurst)
	}
	s, err = New(Config{
		UDPAddr:              "127.0.0.1:0",
		Handler:              StaticA("192.0.2.1"),
		OverflowServfailRate: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.cfg.OverflowServfailRate != -1 || !s.overflow.disabled {
		t.Fatalf("negative rate did not disable: %+v", s.cfg)
	}
}

func packetWithLabels(t *testing.T, id uint16, labels []int) []byte {
	t.Helper()
	var name []byte
	for _, n := range labels {
		if n < 0 || n > 255 {
			t.Fatalf("label %d", n)
		}
		name = append(name, byte(n))
		if n&0xC0 != 0 {
			// Pointer or extended label: one argument octet, then stop.
			// The caller is building a rejected name.
			name = append(name, 0)
			break
		}
		name = append(name, bytes.Repeat([]byte{'a'}, n)...)
	}
	name = append(name, 0)
	pkt := make([]byte, dnswire.HeaderLen+len(name)+4)
	binary.BigEndian.PutUint16(pkt[0:2], id)
	pkt[2] = 0x01 // RD
	binary.BigEndian.PutUint16(pkt[4:6], 1)
	copy(pkt[dnswire.HeaderLen:], name)
	off := dnswire.HeaderLen + len(name)
	binary.BigEndian.PutUint16(pkt[off:off+2], 1)
	binary.BigEndian.PutUint16(pkt[off+2:off+4], 1)
	return pkt
}

func mustEnd(t *testing.T, pkt []byte) int {
	t.Helper()
	end, ok := overflowQuestionEnd(pkt)
	if !ok {
		t.Fatal("question walk failed")
	}
	return end
}
