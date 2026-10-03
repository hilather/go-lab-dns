package config

import "github.com/hilather/go-lab-dns/api/jsonschema"

// SchemaRelPath is the published v1alpha1 JSON Schema, relative to the module root.
const SchemaRelPath = "api/jsonschema/labdns.dev.v1alpha1.json"

// SchemaBytes returns an independent copy of the published v1alpha1 JSON Schema.
// The source artifact under api/jsonschema is embedded directly in the binary.
func SchemaBytes() ([]byte, error) {
	return jsonschema.DesiredState(), nil
}
