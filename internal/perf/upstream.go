package perf

import (
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// FakeUpstream is a localhost UDP/TCP DNS responder used by benches and
// soak/outage tests. It is not a production type.
type FakeUpstream struct {
	udp     net.PacketConn
	tcp     net.Listener
	Packets atomic.Int64

	mu      sync.Mutex
	rcode   model.RCode
	answers []model.RR
	down    bool
}

// StartFakeUpstream binds loopback UDP+TCP on the same port.
func StartFakeUpstream(tb testing.TB) *FakeUpstream {
	tb.Helper()
	f := &FakeUpstream{rcode: model.RCodeNoError}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	pc, err := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		_ = ln.Close()
		tb.Fatal(err)
	}
	f.udp = pc
	f.tcp = ln
	go f.serveUDP()
	go f.serveTCP()
	testutil.Cleanup(tb, func() {
		_ = f.udp.Close()
		_ = f.tcp.Close()
	})
	return f
}

// Addr is host:port for both transports.
func (f *FakeUpstream) Addr() string {
	return f.udp.LocalAddr().String()
}

// SetAnswers replaces the canned NOERROR answer set.
func (f *FakeUpstream) SetAnswers(rrs ...model.RR) {
	f.mu.Lock()
	f.answers = rrs
	f.rcode = model.RCodeNoError
	f.mu.Unlock()
}

// SetRCode forces a response code (answers cleared unless NOERROR).
func (f *FakeUpstream) SetRCode(rc model.RCode) {
	f.mu.Lock()
	f.rcode = rc
	f.mu.Unlock()
}

// SetDown silences the upstream (outage). SetDown(false) recovers.
func (f *FakeUpstream) SetDown(down bool) {
	f.mu.Lock()
	f.down = down
	f.mu.Unlock()
}

func (f *FakeUpstream) snapshot() (down bool, rc model.RCode, answers []model.RR) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.down, f.rcode, append([]model.RR(nil), f.answers...)
}

func (f *FakeUpstream) serveUDP() {
	buf := make([]byte, 4096)
	for {
		n, addr, err := f.udp.ReadFrom(buf)
		if err != nil {
			return
		}
		f.Packets.Add(1)
		out := f.reply(buf[:n])
		if len(out) > 0 {
			_, _ = f.udp.WriteTo(out, addr)
		}
	}
}

func (f *FakeUpstream) serveTCP() {
	for {
		c, err := f.tcp.Accept()
		if err != nil {
			return
		}
		go f.handleTCP(c)
	}
}

func (f *FakeUpstream) handleTCP(c net.Conn) {
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	var hdr [2]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n <= 0 || n > 65535 {
		return
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c, body); err != nil {
		return
	}
	f.Packets.Add(1)
	out := f.reply(body)
	if len(out) == 0 {
		return
	}
	binary.BigEndian.PutUint16(hdr[:], uint16(len(out)))
	_, _ = c.Write(hdr[:])
	_, _ = c.Write(out)
}

func (f *FakeUpstream) reply(in []byte) []byte {
	down, rc, answers := f.snapshot()
	if down {
		return nil
	}
	req, err := dnswire.Parse(in, model.TransportUDP, netip.MustParseAddr("127.0.0.1"))
	if err != nil && (req == nil || !req.HeaderOK) {
		return nil
	}
	res := model.Result{RCode: rc, Answers: answers}
	if rc == "" {
		res.RCode = model.RCodeNoError
	}
	if len(res.Answers) > 0 && res.Answers[0].Name == "" && req != nil {
		for i := range res.Answers {
			res.Answers[i].Name = req.Query.Name
			if res.Answers[i].Class == "" {
				res.Answers[i].Class = model.ClassIN
			}
		}
	}
	out, err := dnswire.Encode(req, res, dnswire.EncodeOpts{})
	if err != nil {
		return nil
	}
	return out
}
