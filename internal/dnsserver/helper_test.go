package dnsserver

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

var (
	loopback = netip.MustParseAddr("127.0.0.1")
	errTest  = errors.New("test handler error")
)

func startServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	if cfg.Handler == nil {
		cfg.Handler = StaticA("192.0.2.1")
	}
	if cfg.UDPAddr == "" && cfg.TCPAddr == "" {
		cfg.UDPAddr = "127.0.0.1:0"
		cfg.TCPAddr = "127.0.0.1:0"
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return s
}

func packA(t *testing.T, name string, id uint16, edns *dnswire.EDNS) []byte {
	t.Helper()
	raw, err := dnswire.PackQuery(id, model.Query{
		Name:  model.Name(name),
		Type:  model.TypeA,
		Class: model.ClassIN,
		RD:    true,
	}, edns)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func exchangeUDP(t *testing.T, addr net.Addr, payload []byte, wait time.Duration) []byte {
	t.Helper()
	c, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	testutil.MustClose(t, c)
	_ = c.SetDeadline(time.Now().Add(wait))
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8192)
	n, err := c.Read(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

func mustExchangeUDP(t *testing.T, addr net.Addr, payload []byte) []byte {
	t.Helper()
	out := exchangeUDP(t, addr, payload, time.Second)
	if out == nil {
		t.Fatal("udp: no response")
	}
	return out
}

func exchangeTCP(t *testing.T, addr net.Addr, payload []byte, wait time.Duration) ([]byte, error) {
	t.Helper()
	c, err := net.Dial("tcp", addr.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(wait))
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	if _, err := c.Write(hdr[:]); err != nil {
		return nil, err
	}
	if _, err := c.Write(payload); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n == 0 {
		return []byte{}, nil
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c, body); err != nil {
		return nil, err
	}
	return body, nil
}

func mustExchangeTCP(t *testing.T, addr net.Addr, payload []byte) []byte {
	t.Helper()
	out, err := exchangeTCP(t, addr, payload, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func rcodeOf(t *testing.T, msg []byte) byte {
	t.Helper()
	if len(msg) < dnswire.HeaderLen {
		t.Fatalf("short message %d", len(msg))
	}
	return msg[3] & 0x0F
}

func hasTC(msg []byte) bool {
	return len(msg) >= 3 && msg[2]&0x02 != 0
}

func hasQR(msg []byte) bool {
	return len(msg) >= 3 && msg[2]&0x80 != 0
}

func qdcountOf(msg []byte) uint16 {
	if len(msg) < 6 {
		return 0
	}
	return uint16(msg[4])<<8 | uint16(msg[5])
}
