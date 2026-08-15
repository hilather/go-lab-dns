package dnsserver

import (
	"context"
	"errors"
	"net"

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
		if !s.acquireInflight() {
			s.cfg.Metrics.IncAdmission("inflight", "")
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.releaseInflight()
			s.handleUDP(pkt, peer)
		}()
	}
}

func (s *Server) handleUDP(pkt []byte, addr net.Addr) {
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
	_, _ = s.udp.WriteTo(payload, addr)
}

func isClosed(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled)
}
