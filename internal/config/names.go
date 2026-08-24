package config

import (
	"strings"
	"unicode"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}

// CanonicalName lower-cases ASCII names, ensures a trailing dot, and expands
// relative owners against origin. Non-ASCII names are rejected (no IDNA yet).
// Label and presentation FQDN length are not capped at RFC 1035 wire maxima
// so lab/QA documents can store over-length names (ADR 0009).
func CanonicalName(s string, origin model.Name) (model.Name, *domainerr.FieldViolation) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", &domainerr.FieldViolation{Code: violationRequired, Message: "name is required"}
	}
	if hasNonASCII(s) {
		return "", &domainerr.FieldViolation{Code: violationNonASCII, Message: "non-ASCII names are rejected in v1alpha1"}
	}
	if s == "@" {
		if origin == "" {
			return "", &domainerr.FieldViolation{Code: violationInvalidName, Message: "\"@\" requires a zone origin"}
		}
		return origin, nil
	}
	s = strings.ToLower(s)
	if !strings.HasSuffix(s, ".") {
		if origin == "." {
			s += "."
		} else if origin != "" {
			orig := string(origin)
			if !strings.HasSuffix(orig, ".") {
				orig += "."
			}
			s = s + "." + orig
		} else {
			s += "."
		}
	}
	if err := checkDNSName(s); err != nil {
		return "", err
	}
	return model.Name(s), nil
}

func checkDNSName(s string) *domainerr.FieldViolation {
	if len(s) == 0 || s[len(s)-1] != '.' {
		return &domainerr.FieldViolation{Code: violationInvalidName, Message: "name must be a fully qualified ASCII name with a trailing dot"}
	}
	if s == "." {
		return nil
	}
	labels := strings.Split(s[:len(s)-1], ".")
	if len(labels) == 0 {
		return &domainerr.FieldViolation{Code: violationInvalidName, Message: "name has no labels"}
	}
	for i, lab := range labels {
		if lab == "" {
			return &domainerr.FieldViolation{Code: violationInvalidName, Message: "name contains an empty label"}
		}
		if lab == "*" {
			if i != 0 {
				return &domainerr.FieldViolation{Code: violationInvalidName, Message: "wildcard \"*\" is only valid as the leftmost label"}
			}
			continue
		}
		for j, c := range lab {
			// Underscore-labels (_sip._tcp, _acme-challenge) are ASCII and required for SRV.
			if c > unicode.MaxASCII || !isLabelChar(c) {
				return &domainerr.FieldViolation{Code: violationInvalidName, Message: "label contains an invalid character"}
			}
			if c == '-' && (j == 0 || j == len(lab)-1) {
				return &domainerr.FieldViolation{Code: violationInvalidName, Message: "label must not start or end with '-'"}
			}
		}
	}
	return nil
}

func isLabelChar(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

func isWildcardOwner(owner string) bool {
	return strings.HasPrefix(owner, "*.") || owner == "*"
}

func validUserID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r > unicode.MaxASCII || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
