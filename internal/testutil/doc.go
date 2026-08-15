// Package testutil provides deterministic clocks, RNGs, and test cleanup helpers.
//
// Production duration math uses Clock.Monotonic; absolute schedules and
// hash-v1 buckets use Clock.Now.
package testutil
