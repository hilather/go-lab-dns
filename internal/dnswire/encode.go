package dnswire

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/miekg/dns"
)

const defaultAdvertisedUDP uint16 = 1232

// PackQuery encodes a client query. Used by tests and later the forwarder.
func PackQuery(id uint16, q model.Query, edns *EDNS) ([]byte, error) {
	m := new(dns.Msg)
	m.Id = id
	m.RecursionDesired = q.RD
	m.CheckingDisabled = q.CD
	m.Question = []dns.Question{{
		Name:   dns.Fqdn(string(q.Name)),
		Qtype:  wireType(q.Type),
		Qclass: wireClass(q.Class),
	}}
	if edns != nil {
		size := edns.UDPSize
		if size == 0 {
			size = defaultAdvertisedUDP
		}
		m.SetEdns0(size, edns.DO)
		if opt := m.IsEdns0(); opt != nil && edns.Version != 0 {
			opt.SetVersion(edns.Version)
		}
	}
	return m.Pack()
}

// Encode packs a model.Result as a DNS response for req.
func Encode(req *Request, result model.Result, opts EncodeOpts) ([]byte, error) {
	if req == nil || !req.HeaderOK {
		return nil, ErrNoHeader
	}
	m, err := buildMsg(req, result, opts)
	if err != nil {
		return nil, err
	}
	return packMaybeTruncate(m, opts)
}

// EncodeError packs a header-echo error response (FORMERR/NOTIMP/REFUSED/…).
func EncodeError(req *Request, rcode model.RCode, opts EncodeOpts) ([]byte, error) {
	if req == nil || !req.HeaderOK {
		return nil, ErrNoHeader
	}
	res := model.Result{RCode: rcode}
	return Encode(req, res, opts)
}

func buildMsg(req *Request, result model.Result, opts EncodeOpts) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.Id = req.ID
	m.Response = true
	m.Opcode = int(req.Opcode)
	m.RecursionDesired = req.Query.RD
	m.RecursionAvailable = result.RA
	m.Authoritative = result.AA
	m.AuthenticatedData = result.AD
	m.CheckingDisabled = result.CD
	m.Rcode = wireRcode(result.RCode)
	if opts.BadVers {
		m.Rcode = dns.RcodeBadVers
	}

	// Echo a question only when one was actually unpacked. A header-only
	// FORMERR (QDCOUNT>0 but no parsed owner) must not invent ". IN A".
	if req.Query.Name != "" {
		m.Question = []dns.Question{{
			Name:   dns.Fqdn(string(req.Query.Name)),
			Qtype:  wireType(req.Query.Type),
			Qclass: wireClass(req.Query.Class),
		}}
	}

	var err error
	if m.Answer, err = toWireRRs(result.Answers); err != nil {
		return nil, err
	}
	if m.Ns, err = toWireRRs(result.Authority); err != nil {
		return nil, err
	}
	if m.Extra, err = toWireRRs(result.Additional); err != nil {
		return nil, err
	}

	if req.HasEDNS || opts.BadVers {
		adv := opts.AdvertisedUDPSize
		if adv == 0 {
			adv = defaultAdvertisedUDP
		}
		m.SetEdns0(adv, req.EDNS.DO)
	}
	return m, nil
}

func packMaybeTruncate(m *dns.Msg, opts EncodeOpts) ([]byte, error) {
	if opts.ForceTruncate {
		// Minimal TC: header + question + OPT. miekg Truncate sets TC.
		limit := MinUDPSize
		if opts.MaxUDPSize > 0 && opts.MaxUDPSize < limit {
			limit = opts.MaxUDPSize
		}
		m.Truncate(limit)
		if !m.Truncated {
			m.Truncated = true
			m.Answer = nil
			m.Ns = nil
			keepOPT(m)
		}
	} else if opts.MaxUDPSize > 0 {
		packed, err := m.Pack()
		if err != nil {
			return nil, err
		}
		if len(packed) > opts.MaxUDPSize {
			m.Truncate(opts.MaxUDPSize)
		} else {
			return packed, nil
		}
	}
	return m.Pack()
}

func keepOPT(m *dns.Msg) {
	var extra []dns.RR
	for _, rr := range m.Extra {
		if _, ok := rr.(*dns.OPT); ok {
			extra = append(extra, rr)
		}
	}
	m.Extra = extra
}

func toWireRRs(rrs []model.RR) ([]dns.RR, error) {
	if len(rrs) == 0 {
		return nil, nil
	}
	out := make([]dns.RR, 0, len(rrs))
	for _, rr := range rrs {
		w, err := toWireRR(rr)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func toWireRR(rr model.RR) (dns.RR, error) {
	name := string(rr.Name)
	if name == "" {
		name = "."
	}
	ttl := uint32(rr.TTL / time.Second)
	class := string(rr.Class)
	if class == "" {
		class = string(model.ClassIN)
	}
	typ := string(rr.Type)
	if typ == "" {
		return nil, fmt.Errorf("%w: empty type", ErrRR)
	}
	pres := fmt.Sprintf("%s %d %s %s %s", dns.Fqdn(name), ttl, class, typ, rr.Data)
	parsed, err := dns.NewRR(pres)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrRR, pres, err)
	}
	return parsed, nil
}

func wireRcode(c model.RCode) int {
	switch c {
	case model.RCodeNoError, "":
		return dns.RcodeSuccess
	case model.RCodeFormErr:
		return dns.RcodeFormatError
	case model.RCodeServFail:
		return dns.RcodeServerFailure
	case model.RCodeNXDomain:
		return dns.RcodeNameError
	case model.RCodeNotImp:
		return dns.RcodeNotImplemented
	case model.RCodeRefused:
		return dns.RcodeRefused
	default:
		// Unknown mnemonic: fail closed to SERVFAIL, not a guessed RCODE.
		return dns.RcodeServerFailure
	}
}

func wireType(t model.RRType) uint16 {
	s := string(t)
	if s == "" {
		return dns.TypeA
	}
	if strings.HasPrefix(s, "TYPE") {
		n, err := strconv.ParseUint(s[4:], 10, 16)
		if err == nil {
			return uint16(n)
		}
	}
	if v, ok := dns.StringToType[s]; ok {
		return v
	}
	return dns.TypeNone
}

func wireClass(c model.RRClass) uint16 {
	if c == "" || c == model.ClassIN {
		return dns.ClassINET
	}
	if strings.HasPrefix(string(c), "CLASS") {
		n, err := strconv.ParseUint(string(c)[5:], 10, 16)
		if err == nil {
			return uint16(n)
		}
	}
	if v, ok := dns.StringToClass[string(c)]; ok {
		return v
	}
	return dns.ClassINET
}

// EffectiveUDPSize is the payload cap for a UDP response to req.
// No EDNS → 512. Client size below 512 is raised to 512 (RFC 6891).
// Client size above maxEDNS is clamped to maxEDNS.
func EffectiveUDPSize(req *Request, maxEDNS uint16) int {
	if req == nil || !req.HasEDNS {
		return MinUDPSize
	}
	n := int(req.EDNS.UDPSize)
	if n < MinUDPSize {
		n = MinUDPSize
	}
	if maxEDNS == 0 {
		maxEDNS = 4096
	}
	if n > int(maxEDNS) {
		n = int(maxEDNS)
	}
	return n
}
