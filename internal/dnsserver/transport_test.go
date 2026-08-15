package dnsserver

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestUDPDropSendsNoPacket(t *testing.T) {
	s := startServer(t, Config{
		Handler: &Stub{RCode: model.RCodeNoError, Hint: HintDrop},
	})
	if out := exchangeUDP(t, s.UDPAddr(), packA(t, "drop.lab.", 1, nil), 200*time.Millisecond); out != nil {
		t.Fatalf("drop sent %d bytes", len(out))
	}
}

func TestUDPTruncateSetsTC(t *testing.T) {
	s := startServer(t, Config{
		Handler: &Stub{
			RCode: model.RCodeNoError,
			Hint:  HintTruncate,
			Answers: []model.RR{{
				Type:  model.TypeA,
				Class: model.ClassIN,
				TTL:   time.Second,
				Data:  "192.0.2.1",
			}},
		},
	})
	out := mustExchangeUDP(t, s.UDPAddr(), packA(t, "tc.lab.", 2, nil))
	if !hasTC(out) {
		t.Fatal("expected TC")
	}
	if !hasQR(out) {
		t.Fatal("expected QR")
	}
}

func TestTCPHintOnUDPDrops(t *testing.T) {
	s := startServer(t, Config{
		Handler: &Stub{RCode: model.RCodeNoError, Hint: HintTCPReset},
	})
	if out := exchangeUDP(t, s.UDPAddr(), packA(t, "tcp-on-udp.lab.", 3, nil), 200*time.Millisecond); out != nil {
		t.Fatal("TCP hint on UDP must drop, not answer")
	}
}

func TestTCPCloseNoResponse(t *testing.T) {
	s := startServer(t, Config{
		Handler: &Stub{RCode: model.RCodeNoError, Hint: HintTCPClose},
	})
	_, err := exchangeTCP(t, s.TCPAddr(), packA(t, "close.lab.", 4, nil), 400*time.Millisecond)
	if err == nil {
		t.Fatal("TCP close should not return a DNS message")
	}
}

func TestTCPResetSendsRST(t *testing.T) {
	s := startServer(t, Config{
		Handler: &Stub{RCode: model.RCodeNoError, Hint: HintTCPReset},
	})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	writeTCP(t, c, packA(t, "rst.lab.", 5, nil))
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	var buf [2]byte
	_, err = io.ReadFull(c, buf[:])
	if !isConnReset(err) {
		t.Fatalf("want ECONNRESET, got %v", err)
	}
}

func isConnReset(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.ECONNRESET
}

func TestTCPHoldThenCloseBounded(t *testing.T) {
	s := startServer(t, Config{
		MaxHold: 40 * time.Millisecond,
		Handler: &Stub{RCode: model.RCodeNoError, Hint: HintHoldThenClose, HoldFor: 20 * time.Millisecond},
	})
	start := time.Now()
	_, err := exchangeTCP(t, s.TCPAddr(), packA(t, "hold.lab.", 6, nil), time.Second)
	if err == nil {
		t.Fatal("hold-then-close should not return a DNS message")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("hold lasted %s", elapsed)
	}
}

func TestTCPTruncateSendsFull(t *testing.T) {
	s := startServer(t, Config{
		Handler: &Stub{
			RCode: model.RCodeNoError,
			Hint:  HintTruncate,
			Answers: []model.RR{{
				Type:  model.TypeA,
				Class: model.ClassIN,
				TTL:   time.Second,
				Data:  "192.0.2.8",
			}},
		},
	})
	out := mustExchangeTCP(t, s.TCPAddr(), packA(t, "full.lab.", 7, nil))
	if hasTC(out) {
		t.Fatal("TC on TCP is fail-closed to a full send; TC must not be set")
	}
	if rcodeOf(t, out) != 0 {
		t.Fatalf("rcode=%d", rcodeOf(t, out))
	}
}

// Regression: the TCP close-watcher must not consume the next query.
func TestTCPDropKeepsConnection(t *testing.T) {
	s := startServer(t, Config{
		TCPIdleTimeout: 400 * time.Millisecond,
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			if q.Name == "drop.lab." {
				return NewResponse(model.Result{RCode: model.RCodeNoError}), HintDrop, nil
			}
			return StaticA("192.0.2.3").ServeDNS(ctx, q)
		}),
	})
	c, err := net.Dial("tcp", s.TCPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	writeTCP(t, c, packA(t, "drop.lab.", 8, nil))
	// Next query on the same connection should still work.
	writeTCP(t, c, packA(t, "ok.lab.", 9, nil))
	out := readTCP(t, c)
	if rcodeOf(t, out) != 0 {
		t.Fatalf("second query rcode=%d", rcodeOf(t, out))
	}
}

func writeTCP(t *testing.T, c net.Conn, payload []byte) {
	t.Helper()
	var hdr [2]byte
	hdr[0] = byte(len(payload) >> 8)
	hdr[1] = byte(len(payload))
	if _, err := c.Write(hdr[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
}

func readTCP(t *testing.T, c net.Conn) []byte {
	t.Helper()
	var hdr [2]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		t.Fatal(err)
	}
	n := int(hdr[0])<<8 | int(hdr[1])
	body := make([]byte, n)
	if _, err := io.ReadFull(c, body); err != nil {
		t.Fatal(err)
	}
	return body
}
