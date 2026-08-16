package auth

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Redacted is the placeholder substituted for secret material.
const Redacted = "[redacted]"

var secretKeys = map[string]bool{
	"secret":        true,
	"secretref":     true,
	"token":         true,
	"password":      true,
	"authorization": true,
	"bearer":        true,
	"credential":    true,
	"credentials":   true,
	"apikey":        true,
	"api_key":       true,
	"accesskey":     true,
	"access_token":  true,
	"refreshtoken":  true,
	"refresh_token": true,
	"privatekey":    true,
	"private_key":   true,
}

// RedactTree copies v with secret-valued keys replaced.
func RedactTree(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			if secretKeys[strings.ToLower(k)] {
				out[k] = Redacted
				continue
			}
			out[k] = RedactTree(child)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = RedactTree(child)
		}
		return out
	default:
		return v
	}
}

// RedactJSON rewrites a JSON document, replacing secret keys.
func RedactJSON(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return raw
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return raw
	}
	out, err := json.Marshal(RedactTree(tree))
	if err != nil {
		return raw
	}
	return out
}

// RedactBytes redacts JSON if possible; otherwise redacts known secret
// substrings in text (YAML export, human diffs, error text).
func RedactBytes(raw []byte) []byte {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return raw
	}
	if trim[0] == '{' || trim[0] == '[' {
		return RedactJSON(raw)
	}
	return []byte(RedactString(string(raw)))
}

// RedactString replaces YAML/text secret assignments and bearer tokens.
func RedactString(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		for key := range secretKeys {
			if strings.Contains(lower, key+":") || strings.Contains(lower, `"`+key+`"`) {
				if idx := strings.Index(line, ":"); idx >= 0 {
					lines[i] = line[:idx+1] + " " + Redacted
					break
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// LooksLikeSecret is a conservative detector for accidental token echo.
func LooksLikeSecret(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(s), "bearer ") {
		return true
	}
	return false
}
