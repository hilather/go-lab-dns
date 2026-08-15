package model

// Stable user-supplied identifiers. IDs are required and immutable within an
// API version; the server does not generate them.

// Name is a DNS name. Normalization (FQDN, lower-case ASCII, trailing dot)
// is applied by config, not by this type.
type Name string

// RecordID identifies an RRset independently of owner text.
type RecordID string

// ZoneID identifies a configured zone.
type ZoneID string

// PolicyID identifies a forwarding or chaos policy.
type PolicyID string

// PoolID identifies an upstream pool.
type PoolID string

// UpstreamID identifies a single upstream endpoint.
type UpstreamID string

// ClientGroupID identifies an access client group.
type ClientGroupID string

// Revision is a content hash: "sha256:" plus lowercase hex of SHA-256 of
// canonical JSON. Hashing is performed by config, not by this package.
type Revision string

// RevisionPrefix is the required prefix of every Revision value.
const RevisionPrefix = "sha256:"

// Generation is a process-local, monotonically increasing snapshot counter.
type Generation uint64
