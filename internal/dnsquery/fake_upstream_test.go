package dnsquery

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

var queryFakeClient = netip.MustParseAddr("127.0.0.1")

type queryFake struct {
	udp     net.PacketConn
	tcp     net.Listener
	Packets atomic.Int64
	mu      sync.Mutex
	rcode   model.RCode
	answers []model.RR
}

func startQueryFake(t *testing.T) *queryFake {
	t.Helper()
	f := &queryFake{rcode: model.RCodeNoError}
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
	testutil.Cleanup(t, func() {
		_ = f.udp.Close()
		_ = f.tcp.Close()
	})
	return f
}

func (f *queryFake) UDPAddr() string { return f.udp.LocalAddr().String() }

func (f *queryFake) setAnswers(rrs ...model.RR) {
	f.mu.Lock()
	f.answers = rrs
	f.mu.Unlock()
}

func (f *queryFake) serveUDP() {
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

func (f *queryFake) serveTCP() {
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
		}(c)
	}
}

func (f *queryFake) reply(pkt []byte) []byte {
	f.mu.Lock()
	rcode := f.rcode
	answers := append([]model.RR(nil), f.answers...)
	f.mu.Unlock()
	req, err := dnswire.Parse(pkt, model.TransportUDP, queryFakeClient)
	if err != nil || req == nil || !req.HeaderOK {
		return nil
	}
	res := model.Result{RCode: rcode, Answers: answers, CD: req.Query.CD}
	out, err := dnswire.Encode(req, res, dnswire.EncodeOpts{})
	if err != nil {
		return nil
	}
	return out
}
