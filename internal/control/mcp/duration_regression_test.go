package mcp

import (
	"math"
	"testing"
	"time"
)

func TestFormatDurationSignedBounds(t *testing.T) {
	for _, duration := range []time.Duration{time.Duration(math.MinInt64), time.Duration(math.MaxInt64), -time.Hour, -time.Millisecond, 0, time.Hour} {
		encoded := formatDuration(duration)
		decoded, err := time.ParseDuration(encoded)
		if err != nil || decoded != duration {
			t.Fatalf("duration %d encoded %q: %d %v", duration, encoded, decoded, err)
		}
	}
}
