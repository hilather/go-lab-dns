package dnsserver

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

func (s *Server) serveTCP() {
	defer s.wg.Done()
	for {
		conn, err := s.tcp.Accept()
		if err != nil {
			if s.ctx.Err() != nil || isClosed(err) {
				return
			}
			continue
		}
		ip := peerFromAddr(conn.RemoteAddr())
		if !s.acquireTCP(conn, ip) {
			s.cfg.Metrics.IncTCP("reject_cap")
			_ = conn.Close()
			continue
		}
		s.cfg.Metrics.IncTCP("accept")
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.releaseTCP(conn, ip)
			s.serveTCPConn(conn, ip)
		}()
	}
}

func (s *Server) acquireTCP(conn net.Conn, ip netip.Addr) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.conns) >= s.cfg.MaxTCPConns {
		return false
	}
	if ip.IsValid() && s.ipCount[ip] >= s.cfg.MaxTCPPerIP {
		return false
	}
	s.conns[conn] = struct{}{}
	s.ipCount[ip]++
	return true
}

func (s *Server) releaseTCP(conn net.Conn, ip netip.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
	n := s.ipCount[ip] - 1
	if n <= 0 {
		delete(s.ipCount, ip)
		return
	}
	s.ipCount[ip] = n
}

func (s *Server) serveTCPConn(raw net.Conn, ip netip.Addr) {
	conn := &leftoverConn{Conn: raw}
	defer func() {
		s.cfg.Metrics.IncTCP("close")
		_ = conn.Close()
	}()
	start := s.now()
	for {
		if s.ctx.Err() != nil {
			return
		}
		if s.now().Sub(start) >= s.cfg.TCPMaxAge {
			return
		}
		remaining := s.cfg.TCPMaxAge - s.now().Sub(start)
		idle := s.cfg.TCPIdleTimeout
		if remaining < idle {
			idle = remaining
		}
		if idle <= 0 {
			return
		}
		_ = conn.SetReadDeadline(s.now().Add(idle))
		var hdr [2]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return
		}
		n := int(binary.BigEndian.Uint16(hdr[:]))
		if n == 0 || n > s.cfg.MaxTCPSize {
			return
		}
		bodyDeadline := s.cfg.TCPReadTimeout
		if remaining := s.cfg.TCPMaxAge - s.now().Sub(start); remaining < bodyDeadline {
			bodyDeadline = remaining
		}
		_ = conn.SetReadDeadline(s.now().Add(bodyDeadline))
		body := make([]byte, n)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}

		if !s.acquireInflight() {
			s.cfg.Metrics.IncAdmission("inflight", "")
			return
		}

		qctx, cancel := context.WithTimeout(s.ctx, s.cfg.QueryTimeout)
		// Cancel the query if the peer FINs while the handler runs.
		// Extra bytes are unread so the next query is not lost.
		stopWatch := s.watchTCPClose(qctx, cancel, conn)
		req, perr := dnswire.Parse(body, model.TransportTCP, ip)
		s.cfg.Metrics.IncParse(parseReason(perr))
		payload, hint, rcode, hold := s.handleQuery(qctx, req, perr, true)
		stopWatch()
		cancel()
		s.releaseInflight()
		s.cfg.Metrics.IncResponse(string(model.TransportTCP), string(rcode), hint.String())

		switch hint {
		case HintDrop:
			// No message; keep the connection until idle/total expiry.
			continue
		case HintTCPReset:
			tcpReset(conn)
			s.cfg.Metrics.IncTCP("reset")
			return
		case HintTCPClose:
			return
		case HintHoldThenClose:
			// Query ctx is already canceled; hold is bounded by server ctx + MaxHold.
			s.holdThenClose(s.ctx, hold)
			return
		default:
			if len(payload) == 0 {
				continue
			}
			if len(payload) > 65535 {
				return
			}
			_ = conn.SetWriteDeadline(s.now().Add(s.cfg.TCPWriteTimeout))
			var whdr [2]byte
			binary.BigEndian.PutUint16(whdr[:], uint16(len(payload)))
			if _, err := conn.Write(whdr[:]); err != nil {
				return
			}
			if _, err := conn.Write(payload); err != nil {
				return
			}
		}
	}
}

func (s *Server) holdThenClose(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	if d > s.cfg.MaxHold {
		d = s.cfg.MaxHold
	}
	ch, stop := s.newTimer(d)
	defer stop()
	select {
	case <-ctx.Done():
	case <-s.ctx.Done():
	case <-ch:
	}
}

// watchTCPClose cancels qcancel when the peer closes during ServeDNS.
// A readable byte is pushed back; it is the next message, not a close.
func (s *Server) watchTCPClose(qctx context.Context, qcancel context.CancelFunc, conn *leftoverConn) func() {
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1)
		for {
			select {
			case <-stop:
				return
			case <-qctx.Done():
				return
			default:
			}
			_ = conn.SetReadDeadline(s.now().Add(25 * time.Millisecond))
			n, err := conn.Read(buf)
			if n > 0 {
				conn.unread(buf[:n])
				return
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			if err != nil {
				qcancel()
			}
			return
		}
	}()
	return func() {
		close(stop)
		_ = conn.SetReadDeadline(s.now())
		<-done
		_ = conn.SetReadDeadline(time.Time{})
	}
}

// leftoverConn returns bytes the close-watcher accidentally consumed.
type leftoverConn struct {
	net.Conn
	mu     sync.Mutex
	prefix []byte
}

func (c *leftoverConn) unread(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prefix = append(append([]byte(nil), b...), c.prefix...)
}

func (c *leftoverConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	if len(c.prefix) > 0 {
		n := copy(p, c.prefix)
		c.prefix = c.prefix[n:]
		c.mu.Unlock()
		return n, nil
	}
	c.mu.Unlock()
	return c.Conn.Read(p)
}

func tcpReset(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = conn.Close()
}
