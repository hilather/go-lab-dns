// Package observability adapts metrics, tracing, and structured logs.
//
// It is a leaf: no domain, snapshot, or control-plane imports. Callers
// increment catalog metrics, emit stable events, and evaluate health
// facts. Telemetry never blocks the DNS path; overflow is dropped and
// counted.
package observability
