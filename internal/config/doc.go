// Package config decodes, normalizes, and schema-validates LabDNS YAML and JSON.
//
// Decode rejects unknown fields at every nesting level. Normalize materializes
// defaults and canonicalizes names (copy-on-write). Validate returns
// domainerr.validation_failed with fieldViolations. Canonical export hashes
// materialized JSON; comments are not preserved.
//
// access.unknownClient defaults to refuse-forward (local answers still serve;
// forwarding is denied). ClientGroup.AllowForward is defaulted to true at
// decode time when the key is omitted; Normalize cannot distinguish an
// explicit false from the Go zero value. Empty clientGroups is valid: local
// zones still answer and nothing is forwarded.
package config
