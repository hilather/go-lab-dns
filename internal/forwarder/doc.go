// Package forwarder selects upstream policy, exchanges queries, and tracks health.
//
// Compile fills snapshot.ForwardingIndex (longest-suffix policies, pools).
// Exchange consumes a pre-selected policy ID and does not rediscover the suffix.
//
// There is no host-resolver fallback: only configured endpoints are dialed.
// FailoverSpec bools are not materialized — the Go zero value means "do not
// fail over / do not TCP-retry". A zero Timeout is not unlimited; Exchange
// uses DefaultExchangeTimeout (2s) for the per-upstream attempt.
//
// Forwarded answers never set AA or AD. CD is passed through. RA is left
// false for the orchestrator.
package forwarder
