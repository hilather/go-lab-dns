package dnsserver

import (
	"context"
	"net/netip"

	"github.com/hilather/go-lab-dns/internal/model"
)

type ctxKey int

const (
	peerKey ctxKey = iota + 1
	transportKey
)

func withPeer(ctx context.Context, addr netip.Addr, tr model.Transport) context.Context {
	ctx = context.WithValue(ctx, peerKey, addr)
	return context.WithValue(ctx, transportKey, tr)
}

// PeerAddr is the client address attached by the listener, if any.
func PeerAddr(ctx context.Context) (netip.Addr, bool) {
	a, ok := ctx.Value(peerKey).(netip.Addr)
	return a, ok
}

// TransportFromContext is the listener transport attached to ctx.
func TransportFromContext(ctx context.Context) (model.Transport, bool) {
	tr, ok := ctx.Value(transportKey).(model.Transport)
	return tr, ok
}

func (s *Server) annotate(ctx context.Context, q *model.Query) context.Context {
	if s.cfg.AcquireSnapshot != nil {
		ctx = s.cfg.AcquireSnapshot(ctx)
	}
	if s.cfg.ClassifySource != nil {
		ctx = s.cfg.ClassifySource(ctx, q)
	}
	return ctx
}
