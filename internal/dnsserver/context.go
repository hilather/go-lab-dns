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
	serverKey
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

// WithServerContext attaches the listener lifetime context so chaos
// delays can ignore the query deadline without ignoring Shutdown.
func WithServerContext(ctx, server context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if server == nil {
		return ctx
	}
	return context.WithValue(ctx, serverKey, server)
}

// ServerContext is the listener context attached by handleQuery, or nil.
func ServerContext(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(serverKey).(context.Context)
	return s
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
