// Package jsonschema embeds published schemas for binaries without a source checkout.
package jsonschema

import _ "embed"

// desiredState is embedded directly from the published schema so no generated
// copy can drift from the configuration contract.
//
//go:embed labdns.dev.v1alpha1.json
var desiredState []byte

// DesiredState returns an independent copy of the published desired-state schema.
func DesiredState() []byte {
	return append([]byte(nil), desiredState...)
}
