package dnswire

import (
	"errors"
	"fmt"

	"github.com/hilather/go-lab-dns/internal/model"
)

// HeaderLen is the DNS message header size in octets (RFC 1035).
const HeaderLen = 12

// MinUDPSize is the RFC 1035 UDP payload limit without EDNS.
const MinUDPSize = 512

// Opcode is a DNS opcode. Only Query is admitted by dnsserver.
type Opcode uint8

const (
	OpcodeQuery  Opcode = 0
	OpcodeIQuery Opcode = 1
	OpcodeStatus Opcode = 2
	OpcodeNotify Opcode = 4
	OpcodeUpdate Opcode = 5
)

func (o Opcode) String() string {
	switch o {
	case OpcodeQuery:
		return "QUERY"
	case OpcodeIQuery:
		return "IQUERY"
	case OpcodeStatus:
		return "STATUS"
	case OpcodeNotify:
		return "NOTIFY"
	case OpcodeUpdate:
		return "UPDATE"
	default:
		return fmt.Sprintf("OPCODE%d", uint8(o))
	}
}

// EDNS is the subset of EDNS(0) the transport needs. No library types.
type EDNS struct {
	Version       uint8
	UDPSize       uint16
	DO            bool
	ExtendedRcode uint16 // OPT EXTENDED-RCODE (RFC 6891); BADVERS is 16
}

// Request is a parsed query plus wire metadata needed to echo a response.
// Query holds the canonical question; ID/Opcode/flags stay here so model
// stays wire-free.
type Request struct {
	Query model.Query

	ID      uint16
	Opcode  Opcode
	QR      bool
	AA      bool
	TC      bool
	RD      bool
	RA      bool
	AD      bool
	CD      bool
	QDCount int

	HasEDNS bool
	EDNS    EDNS

	// HeaderOK is true when the first 12 octets could be read.
	// A parse error with HeaderOK still allows a FORMERR reply.
	HeaderOK bool
}

// EncodeOpts control response packing and truncation.
// Callers that need an EDNS UDP clamp must pass MaxUDPSize:
// EffectiveUDPSize(req, maxEDNS). This type does not re-clamp.
type EncodeOpts struct {
	// AdvertisedUDPSize is the UDP payload we put in our OPT (0 → 1232).
	AdvertisedUDPSize uint16
	// ForceTruncate sets TC and strips answer/authority/additional
	// (except OPT). Used for the chaos truncate action.
	ForceTruncate bool
	// MaxUDPSize is the payload cap applied after packing. 0 skips the cap
	// (TCP). When the packed message exceeds it, TC is set and the
	// message is truncated.
	MaxUDPSize int
	// BadVers writes RCODE BADVERS (RFC 6891): header RCODE 0, OPT
	// EXTENDED-RCODE 16, OPT VERSION 0.
	BadVers bool
}

var (
	// ErrMalformed is returned when miekg/dns cannot unpack the message.
	ErrMalformed = errors.New("dnswire: malformed message")
	// ErrShortHeader is returned when fewer than 12 octets are present.
	ErrShortHeader = errors.New("dnswire: short header")
	// ErrEmpty is returned for a zero-length datagram.
	ErrEmpty = errors.New("dnswire: empty message")
	// ErrNoHeader means Encode was asked to reply without a usable header.
	ErrNoHeader = errors.New("dnswire: no usable request header")
	// ErrRR means a model.RR could not be encoded as presentation RDATA.
	ErrRR = errors.New("dnswire: unencodable resource record")
)

// IsMalformed reports whether err is a parse failure (drop or FORMERR).
func IsMalformed(err error) bool {
	return err != nil && (errors.Is(err, ErrMalformed) || errors.Is(err, ErrShortHeader) || errors.Is(err, ErrEmpty))
}
