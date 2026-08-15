package dnswire

import (
	"fmt"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/miekg/dns"
)

// UpstreamMsg is a parsed upstream response. No library types escape.
type UpstreamMsg struct {
	ID                         uint16
	QR, AA, TC, RD, RA, AD, CD bool
	RCode                      model.RCode
	Answers                    []model.RR
	Authority                  []model.RR
	Additional                 []model.RR
}

// UnpackUpstream unpacks an upstream DNS response. OPT is stripped from
// additional. Unknown RCODE mnemonics stay as the wire name when miekg
// knows them; otherwise they fail closed to SERVFAIL.
func UnpackUpstream(msg []byte) (*UpstreamMsg, error) {
	if len(msg) == 0 {
		return nil, ErrEmpty
	}
	if len(msg) < HeaderLen {
		return nil, ErrShortHeader
	}
	m := new(dns.Msg)
	if err := m.Unpack(msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	u := &UpstreamMsg{
		ID:    m.Id,
		QR:    m.Response,
		AA:    m.Authoritative,
		TC:    m.Truncated,
		RD:    m.RecursionDesired,
		RA:    m.RecursionAvailable,
		AD:    m.AuthenticatedData,
		CD:    m.CheckingDisabled,
		RCode: modelRcode(m.Rcode),
	}
	u.Answers = fromWireRRs(m.Answer)
	u.Authority = fromWireRRs(m.Ns)
	u.Additional = fromWireRRsSkipOPT(m.Extra)
	return u, nil
}

func modelRcode(n int) model.RCode {
	if s, ok := dns.RcodeToString[n]; ok {
		return model.RCode(s)
	}
	return model.RCodeServFail
}

func fromWireRRs(rrs []dns.RR) []model.RR {
	if len(rrs) == 0 {
		return nil
	}
	out := make([]model.RR, 0, len(rrs))
	for _, rr := range rrs {
		if rr == nil {
			continue
		}
		out = append(out, fromWireRR(rr))
	}
	return out
}

func fromWireRRsSkipOPT(rrs []dns.RR) []model.RR {
	if len(rrs) == 0 {
		return nil
	}
	out := make([]model.RR, 0, len(rrs))
	for _, rr := range rrs {
		if rr == nil {
			continue
		}
		if _, ok := rr.(*dns.OPT); ok {
			continue
		}
		out = append(out, fromWireRR(rr))
	}
	return out
}

func fromWireRR(rr dns.RR) model.RR {
	h := rr.Header()
	return model.RR{
		Name:  model.Name(canonicalName(h.Name)),
		Type:  modelType(h.Rrtype),
		Class: modelClass(h.Class),
		TTL:   time.Duration(h.Ttl) * time.Second,
		Data:  rdataPresentation(rr),
	}
}

func rdataPresentation(rr dns.RR) string {
	hs := strings.TrimSpace(rr.Header().String())
	full := strings.TrimSpace(rr.String())
	return strings.TrimSpace(strings.TrimPrefix(full, hs))
}
