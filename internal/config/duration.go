package config

import (
	"encoding/json"
	"strconv"
	"time"
)

// durationFields are JSON/YAML keys whose values use Go time.ParseDuration
// syntax. Numeric JSON values are nanoseconds (encoding/json for time.Duration).
var durationFields = map[string]bool{
	"ttl":                true,
	"negativeTTL":        true,
	"refresh":            true,
	"retry":              true,
	"expire":             true,
	"minimum":            true,
	"timeout":            true,
	"minimumTTL":         true,
	"maximumTTL":         true,
	"maximumNegativeTTL": true,
	"maxDelay":           true,
	"defaultMaxLifetime": true,
	"timeBucket":         true,
	"period":             true,
	"unhealthy":          true,
	"phaseOffset":        true,
	"duration":           true,
	"min":                true,
	"max":                true,
	"hold":               true,
}

func convertDurationStrings(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if durationFields[k] {
				if s, ok := child.(string); ok {
					d, err := time.ParseDuration(s)
					if err == nil {
						x[k] = int64(d)
					}
				}
			}
			convertDurationStrings(child)
		}
	case []any:
		for _, child := range x {
			convertDurationStrings(child)
		}
	}
}

func convertDurationNumbersToStrings(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if durationFields[k] {
				if d, ok := jsonNumberDuration(child); ok {
					x[k] = FormatDuration(d)
				}
			}
			convertDurationNumbersToStrings(child)
		}
	case []any:
		for _, child := range x {
			convertDurationNumbersToStrings(child)
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
	case int:
		return time.Duration(n), true
	default:
		return 0, false
	}
}

// FormatDuration is the canonical duration spelling used in export and hashes.
// It prefers a single whole unit (24h, 5m, 30s, 100ms) over Go's "1h0m0s".
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < 0 {
		return "-" + FormatDuration(-d)
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
