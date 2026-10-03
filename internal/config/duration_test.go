package config

import (
	"math"
	"testing"
	"time"
)

func TestFormatDurationInt64Bounds(t *testing.T) {
	for _, d := range []time.Duration{time.Duration(math.MinInt64), time.Duration(math.MaxInt64), -time.Hour, -time.Nanosecond} {
		got, err := time.ParseDuration(FormatDuration(d))
		if err != nil || got != d {
			t.Fatalf("duration %d did not round-trip: %v %v", d, got, err)
		}
	}
}
