// Package capabilities is the shared capability registry and parity metadata.
//
// REST and MCP adapters register from this package. They must not invent
// paths, tool names, or resource URIs: renaming a row is a compatibility
// change and requires updating the catalog, the generated manifest, and
// docs/implementation-design.md together. Health live/ready are REST-only
// process probes and are not MCP tools. Operator UI route/action bindings
// live on each PARITY_REQUIRED row; RESTOnly probes may also declare UI.
//
// Error helpers map domainerr values to RFC 9457 problem+json status hints
// and JSON-RPC envelopes. They do not start an HTTP or MCP server.
package capabilities
