package observability

// DNSTransport implements the dnsserver.Metrics method set without
// importing dnsserver. cmd/labdns assigns it to dnsserver.Config.Metrics.
type DNSTransport struct {
	R *Registry
}

// NewDNSTransport wraps r. A nil r is a no-op.
func NewDNSTransport(r *Registry) DNSTransport {
	return DNSTransport{R: r}
}

// IncQuery counts an admitted query. transport is "udp" or "tcp".
func (d DNSTransport) IncQuery(transport string) {
	if d.R == nil {
		return
	}
	d.R.Inc(MetricDNSAdmitted, map[string]string{"transport": transport}, 1)
}

// IncParse counts a parse outcome.
func (d DNSTransport) IncParse(result string) {
	if d.R == nil {
		return
	}
	d.R.Inc(MetricDNSParse, map[string]string{"result": result}, 1)
}

// IncAdmission counts an admission decision.
func (d DNSTransport) IncAdmission(result, rcode string) {
	if d.R == nil {
		return
	}
	d.R.Inc(MetricDNSAdmission, map[string]string{"result": result, "rcode": rcode}, 1)
}

// IncResponse counts a finished query.
func (d DNSTransport) IncResponse(transport, rcode, action string) {
	if d.R == nil {
		return
	}
	d.R.Inc(MetricDNSResponses, map[string]string{"transport": transport, "rcode": rcode, "action": action}, 1)
}

// IncTCP counts accept, reject_cap, close, reset.
func (d DNSTransport) IncTCP(event string) {
	if d.R == nil {
		return
	}
	d.R.Inc(MetricDNSTCPEvents, map[string]string{"event": event}, 1)
}
