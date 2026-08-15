package forwarder

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

var fakeClient = netip.MustParseAddr("127.0.0.1")

// fakeUpstream is an httptest-style UDP/TCP DNS server built on dnswire
// (no miekg in this package).
type fakeUpstream struct {
	udp net.PacketConn
	tcp net.Listener

	Packets atomic.Int64
	mu      sync.Mutex
	rcode   model.RCode
	answers []model.RR
	auth    []model.RR
	trunc   bool
	delay   time.Duration
	hang    bool
	handler func(q model.Query) model.Result
}

func startFake(t *testing.T) *fakeUpstream {
	t.Helper()
	f := &fakeUpstream{rcode: model.RCodeNoError}
	// Bind TCP first, then UDP on the same port so UDP-truncate → TCP retry
	// hits this process instead of a different ephemeral port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	pc, err := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		_ = ln.Close()
		t.Fatal(err)
	}
	f.udp = pc
	f.tcp = ln
	go f.serveUDP()
	go f.serveTCP()
	testutil.Cleanup(t, func() { f.Close() })
	return f
}

func (f *fakeUpstream) Close() {
	if f.udp != nil {
		_ = f.udp.Close()
	}
	if f.tcp != nil {
		_ = f.tcp.Close()
	}
}

func (f *fakeUpstream) UDPAddr() string { return f.udp.LocalAddr().String() }
func (f *fakeUpstream) TCPAddr() string { return f.tcp.Addr().String() }

func (f *fakeUpstream) setRCode(c model.RCode) {
	f.mu.Lock()
	f.rcode = c
	f.mu.Unlock()
}

func (f *fakeUpstream) setAnswers(rrs ...model.RR) {
	f.mu.Lock()
	f.answers = rrs
	f.mu.Unlock()
}

func (f *fakeUpstream) setAuth(rrs ...model.RR) {
	f.mu.Lock()
	f.auth = rrs
	f.mu.Unlock()
}

func (f *fakeUpstream) setTruncate(v bool) {
	f.mu.Lock()
	f.trunc = v
	f.mu.Unlock()
}

func (f *fakeUpstream) setHang(v bool) {
	f.mu.Lock()
	f.hang = v
	f.mu.Unlock()
}

func (f *fakeUpstream) serveUDP() {
	buf := make([]byte, 4096)
	for {
		n, addr, err := f.udp.ReadFrom(buf)
		if err != nil {
			return
		}
		f.Packets.Add(1)
		pkt := append([]byte(nil), buf[:n]...)
		go func() {
			out := f.reply(pkt, false)
			if len(out) == 0 {
				return
			}
			_, _ = f.udp.WriteTo(out, addr)
		}()
	}
}

func (f *fakeUpstream) serveTCP() {
	for {
		c, err := f.tcp.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			_ = c.SetDeadline(time.Now().Add(2 * time.Second))
			var hdr [2]byte
			if _, err := io.ReadFull(c, hdr[:]); err != nil {
				return
			}
			n := int(binary.BigEndian.Uint16(hdr[:]))
			if n <= 0 {
				return
			}
			body := make([]byte, n)
			if _, err := io.ReadFull(c, body); err != nil {
				return
			}
			f.Packets.Add(1)
			out := f.reply(body, true)
			if len(out) == 0 {
				return
			}
			binary.BigEndian.PutUint16(hdr[:], uint16(len(out)))
			_, _ = c.Write(hdr[:])
			_, _ = c.Write(out)
		}(c)
	}
}

func (f *fakeUpstream) reply(pkt []byte, tcp bool) []byte {
	f.mu.Lock()
	hang := f.hang
	delay := f.delay
	trunc := f.trunc && !tcp
	rcode := f.rcode
	answers := append([]model.RR(nil), f.answers...)
	auth := append([]model.RR(nil), f.auth...)
	h := f.handler
	f.mu.Unlock()
	if hang {
		return nil
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	req, err := dnswire.Parse(pkt, model.TransportUDP, fakeClient)
	if err != nil || req == nil || !req.HeaderOK {
		return nil
	}
	res := model.Result{RCode: rcode, Answers: answers, Authority: auth, CD: req.Query.CD}
	if h != nil {
		res = h(req.Query)
	}
	opts := dnswire.EncodeOpts{}
	if trunc {
		opts.ForceTruncate = true
		opts.MaxUDPSize = 512
	}
	out, err := dnswire.Encode(req, res, opts)
	if err != nil {
		return nil
	}
	return out
}
