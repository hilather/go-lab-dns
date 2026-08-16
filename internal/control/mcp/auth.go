package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-dns/internal/audit"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func (s *Server) authenticate(r *http.Request) (auth.Actor, error) {
	return auth.Identify(r.Context(), auth.IdentifyIn{
		RemoteAddr:    r.RemoteAddr,
		Authorization: r.Header.Get(headerAuthorization),
	}, s.authn)
}

func bearerToken(h string) (string, bool) { return auth.BearerToken(h) }

func isLoopback(remoteAddr string) bool { return auth.IsLoopback(remoteAddr) }

func (s *Server) authorizeResource(actor auth.Actor, uri string) error {
	cap, ok := capabilities.LookupResource(uri)
	if !ok {
		// Templates (zones/{id}) are not exact matches; fall back to prefix.
		switch {
		case strings.HasPrefix(uri, "labdns://zones/"):
			cap, ok = capabilities.Lookup(capabilities.Zones)
		case strings.HasPrefix(uri, "labdns://records/"):
			cap, ok = capabilities.Lookup(capabilities.Records)
		case strings.HasPrefix(uri, "labdns://chaos/policies/"):
			cap, ok = capabilities.Lookup(capabilities.ChaosPolicies)
		}
		if !ok {
			return nil
		}
	}
	if err := auth.AuthorizeCapability(actor, cap.RequiredScopes, string(cap.ID)); err != nil {
		s.auditDenied(actor, string(cap.ID), err)
		return err
	}
	return nil
}

func (s *Server) authorizeTool(actor auth.Actor, name string) error {
	caps := capabilities.LookupTool(name)
	if len(caps) == 0 {
		return nil
	}
	cap := caps[0]
	if err := auth.AuthorizeCapability(actor, cap.RequiredScopes, string(cap.ID)); err != nil {
		s.auditDenied(actor, string(cap.ID), err)
		return err
	}
	if cap.ID == capabilities.ChaosEmergency {
		enable := name == "dns_chaos_emergency_enable"
		if err := auth.AuthorizeEmergency(actor, enable); err != nil {
			s.auditDenied(actor, string(cap.ID), err)
			return err
		}
	}
	return nil
}

func (s *Server) auditDenied(actor auth.Actor, cap string, err error) {
	code := ""
	if de, ok := domainerr.As(err); ok {
		code = string(de.Code)
	}
	s.emitAudit(context.Background(), audit.Event{
		ActorID:    actor.ID,
		ActorClass: actor.Class,
		Transport:  "mcp",
		Capability: cap,
		Result:     audit.ResultDenied,
		ErrorCode:  code,
	})
}

type auditRecorder interface {
	RecordAudit(context.Context, audit.Event) string
}

func (s *Server) emitAudit(ctx context.Context, ev audit.Event) {
	if rec, ok := s.svc.(auditRecorder); ok {
		rec.RecordAudit(ctx, ev)
		return
	}
	if s.cfg.Auditor != nil {
		_ = s.cfg.Auditor.Emit(ctx, ev)
	}
}
