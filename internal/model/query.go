package model

import (
	"net/netip"
	"time"
)

// Transport is a DNS query or upstream transport. First GA is udp and tcp only.
type Transport string

const (
	TransportUDP Transport = "udp"
	TransportTCP Transport = "tcp"
)

// AllTransports is the closed first-GA transport set. DoT is not a value.
var AllTransports = []Transport{TransportUDP, TransportTCP}

// RCode is a DNS response code mnemonic.
type RCode string

const (
	RCodeNoError  RCode = "NOERROR"
	RCodeFormErr  RCode = "FORMERR"
	RCodeServFail RCode = "SERVFAIL"
	RCodeNXDomain RCode = "NXDOMAIN"
	RCodeNotImp   RCode = "NOTIMP"
	RCodeRefused  RCode = "REFUSED"
)

// Source is how a Result was produced.
type Source string

const (
	SourceExact       Source = "exact"
	SourceWildcard    Source = "wildcard"
	SourceNegative    Source = "negative"
	SourceFallthrough Source = "fallthrough"
	SourceUpstream    Source = "upstream"
	SourceCache       Source = "cache"
)

// Query is a parsed DNS question plus classification inputs.
type Query struct {
	Name      Name       `json:"name"`
	Type      RRType     `json:"type"`
	Class     RRClass    `json:"class"`
	Client    netip.Addr `json:"client"`
	Transport Transport  `json:"transport"`
	RD        bool       `json:"rd"`
	CD        bool       `json:"cd"`
}

// RR is one resource record in presentation form.
type RR struct {
	Name  Name          `json:"name"`
	Type  RRType        `json:"type"`
	Class RRClass       `json:"class"`
	TTL   time.Duration `json:"ttl"`
	Data  string        `json:"data"`
}

// Result is a resolver or forwarder answer shared by later packages.
type Result struct {
	RCode           RCode        `json:"rcode"`
	Answers         []RR         `json:"answers,omitempty"`
	Authority       []RR         `json:"authority,omitempty"`
	Additional      []RR         `json:"additional,omitempty"`
	AA              bool         `json:"aa"`
	RA              bool         `json:"ra"`
	AD              bool         `json:"ad"`
	CD              bool         `json:"cd"`
	Source          Source       `json:"source,omitempty"`
	ZoneID          ZoneID       `json:"zoneId,omitempty"`
	ZoneMode        ZoneMode     `json:"zoneMode,omitempty"`
	WildcardSource  *RecordID    `json:"wildcardSource,omitempty"`
	ClosestEncloser *Name        `json:"closestEncloser,omitempty"`
	Fallthrough     bool         `json:"fallthrough"`
	ForwardingID    PolicyID     `json:"forwardingId,omitempty"`
	UpstreamID      UpstreamID   `json:"upstreamId,omitempty"`
	EDE             *EDE         `json:"ede,omitempty"`
	Explanation     *Explanation `json:"explanation,omitempty"`
}

// Explanation is the resolve:explain payload shell. Later packages fill it.
type Explanation struct {
	Query           Query         `json:"query"`
	ClientGroupID   ClientGroupID `json:"clientGroupId,omitempty"`
	ZoneID          ZoneID        `json:"zoneId,omitempty"`
	ZoneMode        ZoneMode      `json:"zoneMode,omitempty"`
	Source          Source        `json:"source,omitempty"`
	WildcardSource  *RecordID     `json:"wildcardSource,omitempty"`
	ClosestEncloser *Name         `json:"closestEncloser,omitempty"`
	ForwardingID    PolicyID      `json:"forwardingId,omitempty"`
	PoolID          PoolID        `json:"poolId,omitempty"`
	UpstreamID      UpstreamID    `json:"upstreamId,omitempty"`
	Revision        Revision      `json:"revision,omitempty"`
	BaseRCode       RCode         `json:"baseRcode,omitempty"`
	ChaosDisabled   bool          `json:"chaosDisabled,omitempty"`
	ChaosReason     string        `json:"chaosReason,omitempty"`
	ChaosPolicyIDs  []PolicyID    `json:"chaosPolicyIds,omitempty"`
	ChaosActions    []string      `json:"chaosActions,omitempty"`
}
