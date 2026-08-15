package dnsserver

import (
	"context"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// Stub is a test handler. Zero value answers NXDOMAIN with HintSend.
type Stub struct {
	RCode   model.RCode
	Answers []model.RR
	AA      bool
	Hint    TransportHint
	HoldFor time.Duration
	Delay   time.Duration
	Err     error
	ServeFn HandlerFunc
}

// ServeDNS implements Handler.
func (s *Stub) ServeDNS(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
	if s == nil {
		return NewResponse(model.Result{RCode: model.RCodeNXDomain}), HintSend, nil
	}
	if s.ServeFn != nil {
		return s.ServeFn(ctx, q)
	}
	if s.Delay > 0 {
		timer := time.NewTimer(s.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, HintDrop, ctx.Err()
		case <-timer.C:
		}
	}
	rcode := s.RCode
	if rcode == "" {
		rcode = model.RCodeNXDomain
	}
	answers := append([]model.RR(nil), s.Answers...)
	for i := range answers {
		if answers[i].Name == "" && q != nil {
			answers[i].Name = q.Name
		}
		if answers[i].Class == "" {
			answers[i].Class = model.ClassIN
		}
	}
	resp := NewResponse(model.Result{
		RCode:   rcode,
		Answers: answers,
		AA:      s.AA,
	})
	if s.Hint != HintSend {
		_ = resp.SetHint(s.Hint)
	}
	if s.HoldFor > 0 {
		_ = resp.SetHoldFor(s.HoldFor)
	}
	return resp, s.Hint, s.Err
}

// NXDOMAIN returns a stub that answers NXDOMAIN.
func NXDOMAIN() *Stub {
	return &Stub{RCode: model.RCodeNXDomain}
}

// SERVFAIL returns a stub that answers SERVFAIL.
func SERVFAIL() *Stub {
	return &Stub{RCode: model.RCodeServFail}
}

// StaticA answers every query with the given IPv4 presentation address.
func StaticA(ip string) *Stub {
	return &Stub{
		RCode: model.RCodeNoError,
		AA:    true,
		Answers: []model.RR{{
			Type:  model.TypeA,
			Class: model.ClassIN,
			TTL:   30 * time.Second,
			Data:  ip,
		}},
	}
}
