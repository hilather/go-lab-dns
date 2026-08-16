package audit

import (
	"bytes"
	"encoding/json"
	"strings"
)

const redacted = "[redacted]"

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
}

func secretPath(path string) bool {
	p := strings.ToLower(path)
	for key := range secretKeys {
		if p == key || strings.HasSuffix(p, "."+key) || strings.HasSuffix(p, "/"+key) {
			return true
		}
	}
	return false
}

func redactJSON(raw []byte) []byte {
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
	out, err := json.Marshal(redactTree(tree))
	if err != nil {
		return raw
	}
	return out
}

func redactTree(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			if secretKeys[strings.ToLower(k)] {
				out[k] = redacted
				continue
			}
			out[k] = redactTree(child)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = redactTree(child)
		}
		return out
	default:
		return v
	}
}

func redactText(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "bearer ") {
		return redacted
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		for key := range secretKeys {
			if strings.Contains(lower, key+":") {
				if idx := strings.Index(line, ":"); idx >= 0 {
					lines[i] = line[:idx+1] + " " + redacted
					break
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}
