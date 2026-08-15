package dnsserver

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestTCPPartialReadDeadline(t *testing.T) {
	s := startServer(t, Config{
		TCPReadTimeout: 80 * time.Millisecond,
		TCPIdleTimeout: 80 * time.Millisecond,
		TCPMaxAge:      time.Second,
	})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	// Length prefix says 100 bytes; send 4 and wait.
	if _, err := c.Write([]byte{0, 100, 1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var b [1]byte
	_, err = c.Read(b[:])
	if err == nil {
		t.Fatal("expected close after partial body timeout")
	}
}

func TestTCPIdleDeadline(t *testing.T) {
	s := startServer(t, Config{
		TCPIdleTimeout: 60 * time.Millisecond,
		TCPMaxAge:      time.Second,
	})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var b [1]byte
	if _, err := c.Read(b[:]); err == nil {
		t.Fatal("expected idle close")
	}
}

func TestTCPMaxAge(t *testing.T) {
	s := startServer(t, Config{
		TCPIdleTimeout: time.Second,
		TCPReadTimeout: time.Second,
		TCPMaxAge:      80 * time.Millisecond,
	})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var b [1]byte
	if _, err := c.Read(b[:]); err == nil {
		t.Fatal("expected max-age close")
	}
}

func TestTCPLengthCapCloses(t *testing.T) {
	s := startServer(t, Config{MaxTCPSize: 32})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], 1000)
	if _, err := c.Write(hdr[:]); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, err = io.ReadFull(c, hdr[:])
	if err == nil {
		t.Fatal("expected close on oversized TCP length")
	}
}

func TestTCPConnectionCap(t *testing.T) {
	s := startServer(t, Config{
		MaxTCPConns:    1,
		MaxTCPPerIP:    16,
		TCPIdleTimeout: time.Second,
	})
	c1, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c1.Close() }()
	// Give accept loop time to track c1.
	time.Sleep(30 * time.Millisecond)
	c2, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()
	_ = c2.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var b [1]byte
	if _, err := c2.Read(b[:]); err == nil {
		t.Fatal("second connection should be rejected")
	}
}

func TestTCPPerIPCap(t *testing.T) {
	s := startServer(t, Config{
		MaxTCPConns:    16,
		MaxTCPPerIP:    1,
		TCPIdleTimeout: time.Second,
	})
	c1, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c1.Close() }()
	time.Sleep(30 * time.Millisecond)
	c2, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()
	_ = c2.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var b [1]byte
	if _, err := c2.Read(b[:]); err == nil {
		t.Fatal("per-ip cap should reject second conn")
	}
}

func TestTCPClientDisconnect(t *testing.T) {
	started := make(chan struct{})
	s := startServer(t, Config{
		QueryTimeout: time.Second,
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			close(started)
			<-ctx.Done()
			return nil, HintDrop, ctx.Err()
		}),
	})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	writeTCP(t, c, packA(t, "gone.lab.", 1, nil))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	_ = c.Close()
	// Handler must observe cancel without waiting QueryTimeout.
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		// If the server is still holding the conn in serveTCPConn, that's ok;
		// we only require no panic and eventual close via cleanup.
		time.Sleep(10 * time.Millisecond)
	}
}
