// Package mcp is the MCP transport adapter for the shared capability registry.
//
// It wraps the official Go SDK (github.com/modelcontextprotocol/go-sdk) and
// exposes Streamable HTTP at /mcp for protocol 2026-07-28 only. Tools and
// resources are registered from internal/capabilities. Handlers call
// app.Service and contain no domain mutation logic. Errors use
// capabilities.JSONRPCFrom so data.code matches REST.
//
// Health live/ready are not tools. Prompts are read-only templates that point
// at existing tools. Stdio is a developer adapter and is not required in
// production.
//
//go:generate go run ../../../scripts/generate
package mcp
