package dnswire

import (
	"encoding/binary"
	"net/netip"
	"strings"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/miekg/dns"
)

// Parse unpacks a DNS datagram into a model.Query plus wire metadata.
// It never panics. Library types do not escape.
//
// On unpack failure:
//   - len < 12: (*Request, ErrShortHeader) with HeaderOK=false
//   - len == 0: (*Request, ErrEmpty) with HeaderOK=false
//   - len >= 12: Request with ID/flags from the header, HeaderOK=true,
//     and ErrMalformed so the caller can emit FORMERR
func Parse(msg []byte, transport model.Transport, client netip.Addr) (*Request, error) {
	req := &Request{
		Query: model.Query{
			Client:    client,
			Transport: transport,
			Class:     model.ClassIN,
		},
	}
	if len(msg) == 0 {
		return req, ErrEmpty
	}
	if len(msg) < HeaderLen {
		return req, ErrShortHeader
	}

	req.HeaderOK = true
	req.ID = binary.BigEndian.Uint16(msg[0:2])
	bits := binary.BigEndian.Uint16(msg[2:4])
	req.QR = bits&(1<<15) != 0
	req.Opcode = Opcode((bits >> 11) & 0xF)
	req.AA = bits&(1<<10) != 0
	req.TC = bits&(1<<9) != 0
	req.RD = bits&(1<<8) != 0
	req.RA = bits&(1<<7) != 0
	req.AD = bits&(1<<5) != 0
	req.CD = bits&(1<<4) != 0
	req.QDCount = int(binary.BigEndian.Uint16(msg[4:6]))
	req.Query.RD = req.RD
	req.Query.CD = req.CD

	m := new(dns.Msg)
	if err := m.Unpack(msg); err != nil {
		return req, ErrMalformed
	}

	// Prefer unpacked header if Unpack succeeded (compression-safe).
	req.ID = m.Id
	req.QR = m.Response
	req.Opcode = Opcode(m.Opcode)
	req.AA = m.Authoritative
	req.TC = m.Truncated
	req.RD = m.RecursionDesired
	req.RA = m.RecursionAvailable
	req.AD = m.AuthenticatedData
	req.CD = m.CheckingDisabled
	req.QDCount = len(m.Question)
	req.Query.RD = req.RD
	req.Query.CD = req.CD

	if opt := m.IsEdns0(); opt != nil {
		req.HasEDNS = true
		req.EDNS = EDNS{
			Version:       opt.Version(),
			UDPSize:       opt.UDPSize(),
			DO:            opt.Do(),
			ExtendedRcode: uint16(opt.ExtendedRcode()),
		}
	}

	if len(m.Question) > 0 {
		q := m.Question[0]
		req.Query.Name = model.Name(canonicalName(q.Name))
		req.Query.Type = modelType(q.Qtype)
		req.Query.Class = modelClass(q.Qclass)
	}
	return req, nil
}

func canonicalName(n string) string {
	n = strings.ToLower(strings.TrimSpace(n))
	if n == "" {
		return "."
	}
	return dns.Fqdn(n)
}

func modelType(t uint16) model.RRType {
	s := dns.Type(t).String()
	if s == "" {
		return model.RRType("TYPE0")
	}
	return model.RRType(s)
}

func modelClass(c uint16) model.RRClass {
	if c == dns.ClassINET || c == 0 {
		return model.ClassIN
	}
	s := dns.Class(c).String()
	if s == "" {
		return model.RRClass("CLASS0")
	}
	return model.RRClass(s)
}
