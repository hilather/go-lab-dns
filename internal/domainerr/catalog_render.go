package domainerr

import (
	"bytes"
	"encoding/json"
)

// CatalogRelPath is the generated error catalog, relative to the module root.
const CatalogRelPath = "api/errors/v1.json"

// CatalogGeneratedBy is embedded so verify-generated treats the file as generated.
const CatalogGeneratedBy = "scripts/generate; DO NOT EDIT."

// CatalogDoc is the generated error-code document.
type CatalogDoc struct {
	APIVersion  string         `json:"apiVersion"`
	GeneratedBy string         `json:"generatedBy"`
	Codes       []CatalogEntry `json:"codes"`
}

// CatalogEntry is one stable domain error code.
type CatalogEntry struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
}

// CatalogEntries returns the closed first-GA code list in documented order.
func CatalogEntries() []CatalogEntry {
	out := make([]CatalogEntry, len(catalog))
	for i, e := range catalog {
		out[i] = CatalogEntry{Code: string(e.Code), Retryable: e.Retryable}
	}
	return out
}

// RenderCatalog returns pretty-printed JSON for api/errors/v1.json.
func RenderCatalog() ([]byte, error) {
	doc := CatalogDoc{
		APIVersion:  "labdns.dev/errors/v1",
		GeneratedBy: CatalogGeneratedBy,
		Codes:       CatalogEntries(),
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
