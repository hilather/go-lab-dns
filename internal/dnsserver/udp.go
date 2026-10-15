package dnsserver

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

func (s *Server) serveUDP() {
	defer s.wg.Done()
	buf := make([]byte, s.cfg.MaxUDPSize+1)
	for {
		n, addr, err := s.udp.ReadFrom(buf)
		if err != nil {
			if s.ctx.Err() != nil || isClosed(err) {
				return
			}
			continue
		}
		if n > s.cfg.MaxUDPSize {
			s.cfg.Metrics.IncParse("oversize")
			continue
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		peer := addr
		if !s.allowQuery(peerFromAddr(addr)) {
			s.cfg.Metrics.IncAdmission("rate", "")
			continue
		}
		if !s.acquireInflight() {
			// No slot is held. The reply, when sent, is written here.
			s.replyInflightOverflow(pkt, peer)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			release := sync.OnceFunc(s.releaseInflight)
			defer release()
			s.handleUDP(pkt, peer, release)
		}()
	}
}

// handleUDP answers one datagram. release frees the inflight slot; it is
// called before the reply is written, so a client that sends its next query
// as soon as it reads this answer is not dropped at MaxInflight. The cap
// covers parsing and handling, not the UDP write.
func (s *Server) handleUDP(pkt []byte, addr net.Addr, release func()) {
	client := peerFromAddr(addr)
	req, perr := dnswire.Parse(pkt, model.TransportUDP, client)
	s.cfg.Metrics.IncParse(parseReason(perr))

	qctx, cancel := context.WithTimeout(s.ctx, s.cfg.QueryTimeout)
	defer cancel()

	payload, hint, rcode, _ := s.handleQuery(qctx, req, perr, false)
	s.cfg.Metrics.IncResponse(string(model.TransportUDP), string(rcode), hint.String())
	if hint != HintSend || len(payload) == 0 {
		return
	}
	release()
	if s.beforeUDPWrite != nil {
		s.beforeUDPWrite()
	}
	_, _ = s.udp.WriteTo(payload, addr)
}

func isClosed(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled)
}
