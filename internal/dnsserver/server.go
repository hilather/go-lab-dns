package dnsserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Server is a UDP/TCP DNS listener.
type Server struct {
	cfg Config

	ctx    context.Context
	cancel context.CancelFunc

	udp net.PacketConn
	tcp net.Listener

	inflight chan struct{}

	mu      sync.Mutex
	ipCount map[netip.Addr]int
	conns   map[net.Conn]struct{}
	started bool
	stopped bool

	wg sync.WaitGroup
}

// New validates cfg. Start binds and serves.
func New(cfg Config) (*Server, error) {
	cfg = cfg.withDefaults()
	if cfg.Handler == nil {
		return nil, errors.New("dnsserver: Handler is required")
	}
	if cfg.UDPAddr == "" && cfg.TCPAddr == "" {
		return nil, errors.New("dnsserver: at least one of UDPAddr or TCPAddr is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:      cfg,
		ctx:      ctx,
		cancel:   cancel,
		inflight: make(chan struct{}, cfg.MaxInflight),
		ipCount:  make(map[netip.Addr]int),
		conns:    make(map[net.Conn]struct{}),
	}, nil
}

// Start binds listeners and serves in the background.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("dnsserver: already started")
	}
	if s.stopped {
		return errors.New("dnsserver: start after shutdown")
	}
	if s.cfg.UDPAddr != "" {
		pc, err := net.ListenPacket("udp", s.cfg.UDPAddr)
		if err != nil {
			return fmt.Errorf("dnsserver: udp listen: %w", err)
		}
		s.udp = pc
	}
	if s.cfg.TCPAddr != "" {
		ln, err := net.Listen("tcp", s.cfg.TCPAddr)
		if err != nil {
			if s.udp != nil {
				_ = s.udp.Close()
				s.udp = nil
			}
			return fmt.Errorf("dnsserver: tcp listen: %w", err)
		}
		s.tcp = ln
	}
	s.started = true
	if s.udp != nil {
		s.wg.Add(1)
		go s.serveUDP()
	}
	if s.tcp != nil {
		s.wg.Add(1)
		go s.serveTCP()
	}
	return nil
}

// Shutdown stops accepts, cancels in-flight work, and waits up to ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.cancel()
	udp := s.udp
	tcp := s.tcp
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	if tcp != nil {
		_ = tcp.Close()
	}
	if udp != nil {
		_ = udp.Close()
	}
	for _, c := range conns {
		_ = c.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// UDPAddr is the bound UDP address, or nil.
func (s *Server) UDPAddr() net.Addr {
	if s.udp == nil {
		return nil
	}
	return s.udp.LocalAddr()
}

// TCPAddr is the bound TCP address, or nil.
func (s *Server) TCPAddr() net.Addr {
	if s.tcp == nil {
		return nil
	}
	return s.tcp.Addr()
}

func (s *Server) acquireInflight() bool {
	select {
	case s.inflight <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) releaseInflight() {
	select {
	case <-s.inflight:
	default:
	}
}

func (s *Server) encodeOpts(req *dnswire.Request, forceTruncate, badVers bool, tcp bool) dnswire.EncodeOpts {
	opts := dnswire.EncodeOpts{
		AdvertisedUDPSize: s.cfg.AdvertisedEDNSUDP,
		ForceTruncate:     forceTruncate,
		BadVers:           badVers,
	}
	if !tcp {
		opts.MaxUDPSize = dnswire.EffectiveUDPSize(req, s.cfg.MaxEDNSUDPSize)
	}
	return opts
}

func (s *Server) handleQuery(ctx context.Context, req *dnswire.Request, parseErr error, tcp bool) (payload []byte, hint TransportHint, rcode model.RCode, hold time.Duration) {
	defer func() {
		if rec := recover(); rec != nil {
			payload, hint, rcode, hold = s.servfailOrDrop(req, tcp)
		}
	}()
	ad := admit(req, parseErr, s.cfg.MaxQuestions)
	if ad.reason != "ok" {
		s.cfg.Metrics.IncAdmission(ad.reason, string(ad.rcode))
	} else {
		s.cfg.Metrics.IncAdmission("ok", "")
	}
	if ad.drop {
		return nil, HintDrop, "", 0
	}
	if ad.rcode != "" {
		out, err := dnswire.EncodeError(req, ad.rcode, s.encodeOpts(req, false, ad.badVers, tcp))
		if err != nil {
			return nil, HintDrop, ad.rcode, 0
		}
		return out, HintSend, ad.rcode, 0
	}

	s.cfg.Metrics.IncQuery(string(req.Query.Transport))
	q := req.Query
	ctx = withPeer(ctx, q.Client, q.Transport)
	ctx = s.annotate(ctx, &q)

	resp, hint, herr := s.cfg.Handler.ServeDNS(ctx, &q)
	hint = resolveHint(hint, resp)
	hold = s.cfg.MaxHold
	if resp != nil {
		if h := resp.HoldFor(); h > 0 && h < hold {
			hold = h
		}
	}

	if herr != nil {
		if ctx.Err() != nil {
			return nil, HintDrop, "", 0
		}
		// Handler error: fail closed to SERVFAIL.
		out, err := dnswire.EncodeError(req, model.RCodeServFail, s.encodeOpts(req, false, false, tcp))
		if err != nil {
			return nil, HintDrop, model.RCodeServFail, 0
		}
		return out, HintSend, model.RCodeServFail, 0
	}
	if resp == nil && hint == HintSend {
		out, err := dnswire.EncodeError(req, model.RCodeServFail, s.encodeOpts(req, false, false, tcp))
		if err != nil {
			return nil, HintDrop, model.RCodeServFail, 0
		}
		return out, HintSend, model.RCodeServFail, 0
	}

	hint = applyTransportFallback(hint, tcp)
	switch hint {
	case HintDrop, HintTCPClose, HintTCPReset, HintHoldThenClose:
		rcode = ""
		if resp != nil {
			rcode = resp.Result().RCode
		}
		return nil, hint, rcode, hold
	case HintTruncate:
		res := model.Result{RCode: model.RCodeNoError}
		if resp != nil {
			res = resp.Result()
		}
		out, err := dnswire.Encode(req, res, s.encodeOpts(req, true, false, tcp))
		if err != nil {
			return nil, HintDrop, res.RCode, 0
		}
		return out, HintSend, res.RCode, 0
	default:
		res := model.Result{RCode: model.RCodeServFail}
		if resp != nil {
			res = resp.Result()
		}
		out, err := dnswire.Encode(req, res, s.encodeOpts(req, false, false, tcp))
		if err != nil {
			return nil, HintDrop, res.RCode, 0
		}
		return out, HintSend, res.RCode, 0
	}
}

func (s *Server) servfailOrDrop(req *dnswire.Request, tcp bool) ([]byte, TransportHint, model.RCode, time.Duration) {
	if req != nil && req.HeaderOK {
		out, err := dnswire.EncodeError(req, model.RCodeServFail, s.encodeOpts(req, false, false, tcp))
		if err == nil {
			return out, HintSend, model.RCodeServFail, 0
		}
	}
	return nil, HintDrop, model.RCodeServFail, 0
}

func peerFromAddr(addr net.Addr) netip.Addr {
	if addr == nil {
		return netip.Addr{}
	}
	if a, ok := addr.(interface{ AddrPort() netip.AddrPort }); ok {
		return a.AddrPort().Addr().Unmap()
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return netip.Addr{}
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}

func (s *Server) now() time.Time {
	if s.cfg.Clock != nil {
		return s.cfg.Clock.Now()
	}
	return time.Now()
}

func (s *Server) newTimer(d time.Duration) (c <-chan time.Time, stop func() bool) {
	if s.cfg.Clock != nil {
		t := s.cfg.Clock.NewTimer(d)
		return t.C(), t.Stop
	}
	t := time.NewTimer(d)
	return t.C, t.Stop
}
