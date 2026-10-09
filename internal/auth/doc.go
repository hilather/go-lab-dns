// Package auth authenticates management actors and authorizes capability
// and resource scopes shared by REST and MCP.
//
// Frozen first-GA profiles: dev-loopback-unauth (unauthenticated only from
// 127.0.0.1/::1) and bearer (token required for every peer, including
// loopback; ADR 0010). Health live/ready stay unauthenticated in both.
package auth
