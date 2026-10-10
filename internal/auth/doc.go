// Package auth authenticates management actors and authorizes capability
// and resource scopes shared by REST and MCP.
//
// Frozen first-GA profiles: dev-loopback-unauth (unauthenticated only from
// loopback peers: 127.0.0.0/8, ::1, IPv4-mapped 127/8) and bearer (token required for every peer, including
// loopback; ADR 0010). A nil Authenticator rejects every non-probe request,
// bearer or not. An Authenticator that does not report a profile, or that
// reports an empty or unknown one, loses only the loopback-without-a-bearer
// administrator exception; a presented bearer is still checked by that
// Authenticator. An omitted YAML profile normalizes to bearer
// (config.Normalize, NewPolicy; ADR 0011). An authenticator that does not
// report a profile is still not the loopback exception. Health live/ready
// stay unauthenticated in both profiles.
package auth
