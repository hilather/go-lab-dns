// Package cache implements the process-scoped positive and negative DNS cache.
//
// Cache entries are not stored inside a compiled snapshot. Keys include
// Revision so a mutation cannot return a pre-swap local override. TTL
// clamps follow configured bounds; zero MinimumTTL/MaximumTTL means no
// clamp on that side. Get advertises min(storedTTL−elapsed, ExpireAt−now)
// (floor 0) so a MaximumTTL clamp cannot be served past. Chaos hooks
// (bypass, force-miss, serve-stale, treat-expired, skip-put) change the
// request path or the returned copy.
package cache
