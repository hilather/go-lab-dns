// Package dnsquery orchestrates the DNS data-plane query path.
//
// cmd/labdns constructs the orchestrator and installs it as the DNS handler.
// This package must not be imported by the wire listener for domain logic.
package dnsquery
