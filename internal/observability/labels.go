package observability

import "strings"

// forbiddenSet and allowedSet are built once from the catalog tables.
var (
	forbiddenSet = indexStrings(ForbiddenLabels)
	allowedSet   = indexStrings(AllowedLabels)
)

func indexStrings(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, s := range in {
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

// ForbiddenLabel reports whether key is a prohibited default label
// (raw QNAME, client IP, actor, idempotency key, or free-form error text).
func ForbiddenLabel(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	if _, ok := forbiddenSet[k]; ok {
		return true
	}
	// Compound forms such as client_address or raw_qname.
	if strings.Contains(k, "qname") || strings.Contains(k, "client_ip") || strings.Contains(k, "remote_addr") {
		return true
	}
	return false
}

// AllowedLabel reports whether key is in the global allowlist.
func AllowedLabel(key string) bool {
	_, ok := allowedSet[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// CheckLabels validates labels against the catalog allowlist for metric.
// Unknown metrics, forbidden keys, and keys not declared on the metric fail.
func CheckLabels(metric string, labels map[string]string) error {
	def, ok := LookupMetric(metric)
	if !ok {
		return labelError("unknown_metric")
	}
	return checkLabelsDef(def, labels)
}

func checkLabelsDef(def MetricDef, labels map[string]string) error {
	allowed := make(map[string]struct{}, len(def.Labels))
	for _, l := range def.Labels {
		allowed[l] = struct{}{}
	}
	for k := range labels {
		if ForbiddenLabel(k) {
			return labelError("forbidden_label")
		}
		if _, ok := allowed[k]; !ok {
			return labelError("unknown_label")
		}
	}
	return nil
}

type labelError string

func (e labelError) Error() string { return string(e) }

// LabelReason is the bounded drop reason for a rejected sample.
func LabelReason(err error) string {
	if err == nil {
		return ""
	}
	if r, ok := err.(labelError); ok {
		return string(r)
	}
	return "invalid"
}

// QTypeClass collapses a QTYPE mnemonic to a bounded class.
func QTypeClass(qtype string) string {
	switch strings.ToUpper(strings.TrimSpace(qtype)) {
	case "A", "AAAA", "CNAME", "TXT", "MX", "SRV", "PTR", "NS", "SOA", "CAA", "SVCB", "HTTPS":
		return strings.ToUpper(qtype)
	case "":
		return "empty"
	default:
		return "other"
	}
}

// ClientGroupClass is the privacy-safe client class (not a group id).
func ClientGroupClass(known, allowForward bool) string {
	if !known {
		return "unknown"
	}
	if !allowForward {
		return "local_only"
	}
	return "known"
}

// SourceClass collapses a resolution source to a bounded label.
func SourceClass(source string) string {
	s := strings.ToLower(strings.TrimSpace(source))
	switch s {
	case "exact", "wildcard", "negative", "fallthrough", "upstream", "cache":
		return s
	case "":
		return "unknown"
	default:
		return "other"
	}
}
