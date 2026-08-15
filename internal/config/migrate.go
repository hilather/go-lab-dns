package config

import (
	"bytes"
	"encoding/json"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"gopkg.in/yaml.v3"
)

// CurrentAPIVersion is the only shipped config version.
const CurrentAPIVersion = model.APIVersionV1Alpha1

// Migrator upgrades a document from one apiVersion to another.
// Only v1alpha1 exists; this interface is the extension point for later versions.
type Migrator interface {
	FromVersion() string
	ToVersion() string
	Migrate(raw []byte) ([]byte, error)
}

// Migrations returns registered migrators. Empty until a version after v1alpha1 exists.
func Migrations() []Migrator {
	return nil
}

// PeekAPIVersion reads apiVersion without applying defaults or validation.
func PeekAPIVersion(raw []byte) string {
	raw = stripBOM(bytes.TrimSpace(raw))
	if len(raw) == 0 {
		return ""
	}
	var hdr struct {
		APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	}
	if looksLikeJSON(raw) {
		_ = json.Unmarshal(raw, &hdr)
		return hdr.APIVersion
	}
	_ = yaml.Unmarshal(raw, &hdr)
	return hdr.APIVersion
}

// MigrateToCurrent is a no-op for v1alpha1 and rejects unknown versions.
func MigrateToCurrent(raw []byte) ([]byte, error) {
	ver := PeekAPIVersion(raw)
	if ver == "" || ver == CurrentAPIVersion {
		return raw, nil
	}
	return nil, domainerr.UnsupportedProtocolVersion("unsupported config apiVersion " + ver).
		WithRemediation("only " + CurrentAPIVersion + " is implemented; convert the document or use a newer LabDNS").
		WithViolations(domainerr.FieldViolation{
			Path:    "apiVersion",
			Code:    violationUnsupportedVersion,
			Message: "no migrator from " + ver + " to " + CurrentAPIVersion,
		})
}
