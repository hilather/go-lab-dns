// Package model holds canonical LabDNS domain types.
//
// Types in this package must not import wire, MCP, HTTP, snapshot, or
// telemetry packages. IDs are user-supplied. Validation, YAML decode, and
// default materialization belong in config.
package model
