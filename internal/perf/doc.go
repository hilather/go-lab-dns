// Package perf is the PERF-001 harness: in-process query paths, bounded
// delay/soak/flood checks, and pinned environment metadata for benches.
//
// Absolute latency and QPS numbers are not CI gates (hardware varies).
// Tests assert bounds, completeness under swap, and leak ceilings.
package perf
