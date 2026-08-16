package app

import (
	"context"

	"github.com/hilather/go-lab-dns/internal/audit"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func (s *App) recordAudit(ctx context.Context, ev audit.Event) string {
	if s == nil || s.audit == nil {
		return ""
	}
	if ev.Result == "" {
		ev.Result = audit.ResultOK
	}
	return s.audit.Emit(ctx, ev).ID
}

func (s *App) QueryAudit(ctx context.Context, actor Actor, in AuditQuery) (*AuditList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if s.audit == nil {
		return &AuditList{}, nil
	}
	return &AuditList{Events: s.audit.List(in.Limit)}, nil
}

func (s *App) GetAudit(ctx context.Context, actor Actor, id string) (*AuditEvent, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if s.audit == nil {
		return nil, domainerr.NotFound("audit event " + id + " not found")
	}
	ev, ok := s.audit.Get(id)
	if !ok {
		return nil, domainerr.NotFound("audit event " + id + " not found")
	}
	return &ev, nil
}
