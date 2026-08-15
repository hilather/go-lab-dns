// Package resolver implements exact, wildcard, CNAME, and negative DNS answers.
//
// Compile fills snapshot.ZoneIndex (existence tree, RRsets, wildcards).
// Resolve consumes a pre-selected zone ID and does not rediscover the zone.
//
// Local flag rules: AA only for authoritative local/negative answers; AD is
// never set; CD is cleared; RA is left unset. Overlay misses and overlay
// CNAME targets that leave local data set Fallthrough and do not forward.
package resolver
