package dnsserver

// Metrics are transport counters. Labels must stay bounded: transport,
// rcode, action, reason. Implementations must not add QNAME or client IP.
type Metrics interface {
	// IncQuery counts an admitted query. transport is "udp" or "tcp".
	IncQuery(transport string)
	// IncParse counts a parse outcome: "ok", "malformed", "empty", "short", "oversize".
	IncParse(result string)
	// IncAdmission counts an admission decision. result is "ok" or a reason
	// ("opcode", "qdcount", "class", "qtype", "edns-version", "qr", "inflight").
	// rcode is the DNS mnemonic or "" when the packet is dropped.
	IncAdmission(result, rcode string)
	// IncResponse counts a finished query. action is TransportHint.String().
	IncResponse(transport, rcode, action string)
	// IncTCP counts accept, reject_cap, close, reset.
	IncTCP(event string)
}

// NopMetrics discards all observations.
type NopMetrics struct{}

func (NopMetrics) IncQuery(string)                    {}
func (NopMetrics) IncParse(string)                    {}
func (NopMetrics) IncAdmission(string, string)        {}
func (NopMetrics) IncResponse(string, string, string) {}
func (NopMetrics) IncTCP(string)                      {}
