// Package cache implements the process-scoped positive and negative DNS cache.
//
// Cache entries are not stored inside a compiled snapshot. Keys include
// Revision so a mutation cannot return a pre-swap local override. TTL
// clamps follow configured bounds; zero MinimumTTL/MaximumTTL means no
// clamp on that side. Get returns a copy whose RR TTLs are remaining
// time (floor 0). Chaos hooks (bypass, force-miss, serve-stale, skip-put)
// change the request path or the returned copy.
package cache
