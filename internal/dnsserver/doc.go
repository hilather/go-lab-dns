// Package dnsserver owns UDP and TCP DNS listeners, admission, and
// transport actions. It does not import snapshot, resolver, forwarder, or
// chaos and embeds no resolution policy.
//
// # Handler
//
//	ServeDNS(ctx, *model.Query) (*Response, TransportHint, error)
//
// ctx is canceled on graceful shutdown, QueryTimeout, TCP max-age, or a
// detected TCP peer close. The handler must return promptly after cancel.
//
// # Response ownership
//
// ServeDNS may mutate a Response (SetHint, SetHoldFor) until it returns.
// The server then takes ownership and calls Release. Later transport
// actions fail with ErrReleased and are ignored.
//
// The returned TransportHint is authoritative when it is not HintSend.
// If the return value is HintSend, a non-Send hint stored on Response
// during ServeDNS is used.
//
// # Transport hints
//
//	HintSend          write the encoded response
//	HintDrop          write nothing (UDP: silence; TCP: no message, stay up)
//	HintTruncate      UDP: valid TC response; TCP: Send full (TC is a UDP signal)
//	HintTCPClose      TCP: FIN without a DNS message; UDP: Drop
//	HintTCPReset      TCP: RST without a DNS message; UDP: Drop
//	HintHoldThenClose TCP: wait up to MaxHold then close; UDP: Drop
//
// Unknown hints are Drop. TCP-only hints on UDP are Drop so a mis-wired
// chaos action cannot deliver a successful UDP answer.
//
// # Admission
//
//	empty / short / unparseable without header → drop
//	malformed with header                       → FORMERR
//	QR=1                                        → drop
//	opcode ≠ QUERY                              → NOTIMP
//	QDCOUNT = 0 or > MaxQuestions (default 1)   → FORMERR
//	QCLASS ≠ IN                                 → NOTIMP
//	AXFR / IXFR                                 → NOTIMP
//	EDNS version ≠ 0                            → BADVERS (header RCODE 0 + OPT EXTENDED-RCODE 16)
//	UDP length > MaxUDPSize                     → drop
//	TCP length prefix > MaxTCPSize              → close
//
// # Hooks
//
// AcquireSnapshot and ClassifySource may annotate ctx after admission.
// The transport does not inspect the annotation.
package dnsserver
