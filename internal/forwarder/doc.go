// Package forwarder selects upstream policy, exchanges queries, and tracks health.
//
// Compile fills snapshot.ForwardingIndex (longest-suffix policies, pools).
// Exchange consumes a pre-selected policy ID and does not rediscover the suffix.
//
// There is no host-resolver fallback: only configured endpoints are dialed.
// FailoverSpec bools are not materialized — the Go zero value means "do not
// fail over / do not TCP-retry". A zero Timeout is not unlimited; Exchange
// uses DefaultExchangeTimeout (500ms) for the per-upstream attempt so a
// 2s query timeout can still cover a second try when OnTimeout is set.
// Deadlines stack: parent ctx is total, DefaultConnectTimeout (250ms,
// capped by the attempt) is Dial, remaining attempt time is exchange.
//
// Forwarded answers never set AA or AD. CD is passed through. RA is left
// false for the orchestrator.
package forwarder
