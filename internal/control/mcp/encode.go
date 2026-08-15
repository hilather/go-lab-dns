package mcp

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// durationKeys match REST: model JSON numbers are nanoseconds.
var durationKeys = map[string]bool{
	"ttl": true, "negativeTTL": true, "refresh": true, "retry": true,
	"expire": true, "minimum": true, "timeout": true, "minimumTTL": true,
	"maximumTTL": true, "maximumNegativeTTL": true, "maxDelay": true,
	"defaultMaxLifetime": true, "timeBucket": true, "period": true,
	"unhealthy": true, "phaseOffset": true, "duration": true, "min": true,
	"max": true, "hold": true,
}

func marshalAPI(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	rewriteDurations(tree)
	return json.Marshal(tree)
}

func asStructured(v any) (any, error) {
	raw, err := marshalAPI(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func rewriteDurations(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if durationKeys[k] {
				if d, ok := jsonNumberDuration(child); ok {
					x[k] = formatDuration(d)
					continue
				}
			}
			rewriteDurations(child)
		}
	case []any:
		for _, child := range x {
			rewriteDurations(child)
		}
	}
}

func jsonNumberDuration(v any) (time.Duration, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return time.Duration(i), true
	case int64:
		return time.Duration(n), true
	case float64:
		return time.Duration(n), true
	default:
		return 0, false
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < 0 {
		return "-" + formatDuration(-d)
	}
	if d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	if d%time.Minute == 0 {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	if d%time.Millisecond == 0 {
		return strconv.FormatInt(int64(d/time.Millisecond), 10) + "ms"
	}
	if d%time.Microsecond == 0 {
		return strconv.FormatInt(int64(d/time.Microsecond), 10) + "us"
	}
	return strconv.FormatInt(int64(d), 10) + "ns"
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTimePtr(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
	}
	if err != nil {
		return nil, domainerr.ValidationFailed("invalid timestamp",
			domainerr.FieldViolation{Path: "expiresAt", Code: "invalid_value", Message: "expiresAt must be RFC3339"})
	}
	t = t.UTC()
	return &t, nil
}
