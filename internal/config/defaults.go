package config

import "time"

// Materialized v1alpha1 defaults. Zero values on the Go types are not these
// values until Decode/Normalize run.
const (
	DefaultTTL                  = 30 * time.Second
	DefaultNegativeTTL          = 10 * time.Second
	DefaultDNSAddress           = ":5353"
	DefaultMgmtAddress          = ":8080"
	DefaultRESTPath             = "/v1"
	DefaultMCPPath              = "/mcp"
	MaxDocumentBytes            = 1 << 20
	MinTimeBucket               = time.Second
	maxFQDNPresentation         = 254
	maxDNSLabel                 = 63
	dnameTypeCode               = 39
	violationUnknownField       = "unknown_field"
	violationRequired           = "required"
	violationInvalidValue       = "invalid_value"
	violationDuplicateID        = "duplicate_id"
	violationUnresolved         = "unresolved_reference"
	violationInvalidName        = "invalid_name"
	violationNonASCII           = "non_ascii_name"
	violationCNAMECoexist       = "cname_coexistence"
	violationCNAMELoop          = "cname_loop"
	violationWildcardNS         = "wildcard_ns"
	violationWildcardDNAME      = "wildcard_dname"
	violationTimeBucket         = "time_bucket_too_small"
	violationForwardLoop        = "forward_loop"
	violationConflict           = "conflicting_transport_actions"
	violationInvalidPhase       = "invalid_phase"
	violationMissingExpiry      = "missing_expiry"
	violationDelayCap           = "delay_exceeds_cap"
	violationAltAddr            = "alternate_address_not_allowed"
	violationTooLarge           = "document_too_large"
	violationEmptyID            = "empty_id"
	violationInvalidType        = "invalid_rrtype"
	violationInvalidCIDR        = "invalid_cidr"
	violationInvalidEndpoint    = "invalid_endpoint"
	violationInvalidTransport   = "invalid_transport"
	violationDupRRset           = "duplicate_rrset"
	violationUnsupportedVersion = "unsupported_version"
	violationProtected          = "protected_object"
)
