package model

import "time"

// ZoneMode is the ownership mode of a configured zone.
type ZoneMode string

const (
	ZoneModeAuthoritative ZoneMode = "authoritative"
	ZoneModeOverlay       ZoneMode = "overlay"
)

// RRType is a DNS type mnemonic or TYPE<n> form.
type RRType string

// First-GA record types plus the generic RDATA escape hatch.
const (
	TypeA     RRType = "A"
	TypeAAAA  RRType = "AAAA"
	TypeCNAME RRType = "CNAME"
	TypeTXT   RRType = "TXT"
	TypeMX    RRType = "MX"
	TypeSRV   RRType = "SRV"
	TypePTR   RRType = "PTR"
	TypeCAA   RRType = "CAA"
	TypeNS    RRType = "NS"
	TypeSOA   RRType = "SOA"
	TypeSVCB  RRType = "SVCB"
	TypeHTTPS RRType = "HTTPS"
)

// FirstGARRTypes is the closed first-GA mnemonic set. Generic RDATA uses
// TYPE<n> rather than a mnemonic in this list.
var FirstGARRTypes = []RRType{
	TypeA, TypeAAAA, TypeCNAME, TypeTXT, TypeMX, TypeSRV,
	TypePTR, TypeCAA, TypeNS, TypeSOA, TypeSVCB, TypeHTTPS,
}

// RRClass is a DNS class. First GA serves IN only.
type RRClass string

const ClassIN RRClass = "IN"

// Zone is an authoritative or overlay namespace.
type Zone struct {
	ID          ZoneID   `json:"id"`
	Name        Name     `json:"name"`
	Mode        ZoneMode `json:"mode"`
	SOA         *SOA     `json:"soa,omitempty"`
	Nameservers []Name   `json:"nameservers,omitempty"`
	Records     []Record `json:"records"`
}

// SOA is start-of-authority data. Serial is "auto" or a decimal serial.
type SOA struct {
	Primary       Name          `json:"primary"`
	Administrator Name          `json:"administrator"`
	Serial        string        `json:"serial"`
	Refresh       time.Duration `json:"refresh"`
	Retry         time.Duration `json:"retry"`
	Expire        time.Duration `json:"expire"`
	Minimum       time.Duration `json:"minimum,omitempty"`
}

// Record is one configured RRset.
type Record struct {
	ID              RecordID      `json:"id"`
	Owner           string        `json:"owner"`
	Type            RRType        `json:"type"`
	TTL             time.Duration `json:"ttl"`
	Values          []string      `json:"values,omitempty"`
	GenericRDATA    *GenericRDATA `json:"genericRdata,omitempty"`
	ChaosPolicyRefs []PolicyID    `json:"chaosPolicyRefs,omitempty"`
}

// GenericRDATA is a presentation-format escape hatch for types without a
// first-GA mnemonic. TypeCode is the wire type number.
type GenericRDATA struct {
	TypeCode     uint16 `json:"typeCode"`
	Presentation string `json:"presentation"`
}
