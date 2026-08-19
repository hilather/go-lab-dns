package rest

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/model"
)

// OpenAPIRelPath is the generated OpenAPI document, relative to the module root.
const OpenAPIRelPath = "api/openapi/v1.json"

// OpenAPIVersion is the OpenAPI specification version of the generated file.
const OpenAPIVersion = "3.1.0"

// RenderOpenAPI builds OpenAPI 3.1 JSON from the capability registry and model.Spec.
func RenderOpenAPI() ([]byte, error) {
	doc := map[string]any{
		"openapi": OpenAPIVersion,
		"info": map[string]any{
			"title":          "LabDNS Management API",
			"version":        capabilities.VersionTag,
			"description":    "REST adapter for the shared LabDNS capability registry. Generated from internal/capabilities and model.Spec. Do not edit by hand.",
			"x-generated-by": "internal/control/rest.RenderOpenAPI; DO NOT EDIT.",
		},
		"servers": []any{
			map[string]any{"url": "/", "description": "Management listener (default address " + DefaultAddr + ")"},
		},
		"tags":       openAPITags(),
		"paths":      openAPIPaths(),
		"components": openAPIComponents(),
		"security": []any{
			map[string]any{"bearerAuth": []any{}},
			map[string]any{"cookieAuth": []any{}},
		},
	}
	return marshalSorted(doc)
}

func openAPITags() []any {
	seen := map[string]bool{}
	var tags []any
	for _, c := range capabilities.All() {
		name := tagFor(c)
		if seen[name] {
			continue
		}
		seen[name] = true
		tags = append(tags, map[string]any{"name": name, "description": c.Description})
	}
	return tags
}

func tagFor(c capabilities.Capability) string {
	s := string(c.ID)
	if i := strings.IndexByte(s, '.'); i > 0 {
		return s[:i]
	}
	return s
}

func openAPIPaths() map[string]any {
	paths := map[string]any{}
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			item, _ := paths[b.Path].(map[string]any)
			if item == nil {
				item = map[string]any{}
			}
			item[strings.ToLower(b.Method)] = openAPIOperation(c, b)
			paths[b.Path] = item
		}
	}
	return paths
}

func openAPIOperation(c capabilities.Capability, b capabilities.RESTBinding) map[string]any {
	op := map[string]any{
		"operationId": operationID(b),
		"tags":        []any{tagFor(c)},
		"summary":     c.Title,
		"description": c.Description,
		"parameters":  pathParameters(b.Path),
	}
	switch c.ID {
	case capabilities.HealthLive, capabilities.HealthReady, capabilities.UIAssets:
		op["security"] = []any{}
	case capabilities.Session:
		if strings.EqualFold(b.Method, "POST") || strings.EqualFold(b.Method, "GET") {
			op["security"] = []any{
				map[string]any{},
				map[string]any{"bearerAuth": []any{}},
				map[string]any{"cookieAuth": []any{}},
			}
		}
	}
	if len(c.RequiredScopes) > 0 {
		op["x-required-scopes"] = append([]string(nil), c.RequiredScopes...)
	}
	op["x-capability-id"] = string(c.ID)
	op["x-mutating"] = c.Mutating
	op["x-idempotent"] = c.Idempotent
	if c.InputSchema != nil && strings.EqualFold(b.Method, "POST") {
		op["requestBody"] = map[string]any{
			"required": !optionalBody(c.ID),
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": schemaRefOrObject(c.InputSchema.Name),
				},
			},
		}
	}
	if strings.HasPrefix(b.Path, "/v1/state:export") {
		params, _ := op["parameters"].([]any)
		params = append(params, map[string]any{
			"name":        "format",
			"in":          "query",
			"required":    false,
			"description": "Export encoding. Omitted or yaml returns canonical YAML (application/yaml). json returns a metadata wrapper.",
			"schema":      map[string]any{"type": "string", "enum": []any{"yaml", "json"}, "default": "yaml"},
		})
		op["parameters"] = params
	}
	if needsPagination(b.Path) {
		params, _ := op["parameters"].([]any)
		params = append(params,
			map[string]any{"name": "cursor", "in": "query", "schema": map[string]any{"type": "string"}},
			map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 0}},
		)
		op["parameters"] = params
	}
	if b.Path == "/v1/audit" {
		params, _ := op["parameters"].([]any)
		params = append(params,
			map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 0}},
		)
		op["parameters"] = params
	}
	if strings.HasPrefix(b.Path, "/v1/") && !strings.EqualFold(b.Method, "GET") && !strings.EqualFold(b.Method, "HEAD") {
		params, _ := op["parameters"].([]any)
		params = append(params, map[string]any{
			"name":        auth.CSRFHeader,
			"in":          "header",
			"required":    false,
			"description": "required when authenticating with cookie labdns_session; ignored for Authorization: Bearer.",
			"schema":      map[string]any{"type": "string"},
		})
		op["parameters"] = params
	}
	op["responses"] = openAPIResponses(c, b)
	return op
}

func optionalBody(id capabilities.ID) bool {
	switch id {
	case capabilities.StateReset, capabilities.CacheFlush, capabilities.ChaosActivate,
		capabilities.ChaosSetExpiry, capabilities.ChaosEmergency:
		return true
	default:
		return false
	}
}

func needsPagination(path string) bool {
	switch path {
	case "/v1/zones", "/v1/zones/{zoneId}/records":
		return true
	default:
		return false
	}
}

func operationID(b capabilities.RESTBinding) string {
	p := strings.TrimPrefix(b.Path, "/")
	p = strings.ReplaceAll(p, "/", "_")
	p = strings.ReplaceAll(p, "{", "")
	p = strings.ReplaceAll(p, "}", "")
	p = strings.ReplaceAll(p, ":", "_")
	p = strings.ReplaceAll(p, "-", "_")
	return strings.ToLower(b.Method) + "_" + p
}

func pathParameters(path string) []any {
	var out []any
	for _, seg := range compilePath(path) {
		if seg.wild == "" {
			continue
		}
		out = append(out, map[string]any{
			"name":     seg.wild,
			"in":       "path",
			"required": true,
			"schema":   map[string]any{"type": "string"},
		})
	}
	if out == nil {
		return []any{}
	}
	return out
}

func openAPIResponses(c capabilities.Capability, b capabilities.RESTBinding) map[string]any {
	if c.ID == capabilities.StateExport {
		return map[string]any{
			"200": map[string]any{
				"description": "Canonical export. Default (format omitted or yaml) is application/yaml. format=json is a wrapper object.",
				"content": map[string]any{
					"application/yaml": map[string]any{
						"schema": map[string]any{
							"type":        "string",
							"description": "Canonical YAML document (default success path).",
						},
					},
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/Export"},
					},
				},
			},
			"default": map[string]any{
				"description": "application/problem+json domain error",
				"content": map[string]any{
					capabilities.ProblemContentType: map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/Problem"},
					},
				},
			},
		}
	}
	success := "200"
	successSchema := map[string]any{"type": "object"}
	if c.ID == capabilities.CacheFlush || (c.ID == capabilities.Session && strings.EqualFold(b.Method, "DELETE")) {
		success = "204"
	}
	if c.OutputSchema != nil {
		successSchema = schemaRefOrObject(c.OutputSchema.Name)
	} else {
		switch c.ID {
		case capabilities.HealthLive, capabilities.HealthReady:
			successSchema = map[string]any{"$ref": "#/components/schemas/Health"}
		case capabilities.SchemaConfig:
			successSchema = map[string]any{"type": "object", "description": "Published v1alpha1 JSON Schema"}
		case capabilities.DocsDNSSemantics, capabilities.DocsChaosSafety:
			successSchema = map[string]any{"type": "string"}
		}
	}
	contentType := "application/json"
	if c.ID == capabilities.DocsDNSSemantics || c.ID == capabilities.DocsChaosSafety {
		contentType = "text/markdown"
	}
	if c.ID == capabilities.SchemaConfig {
		contentType = "application/schema+json"
	}
	resp := map[string]any{
		"description": c.Title,
	}
	if success != "204" {
		resp["content"] = map[string]any{
			contentType: map[string]any{"schema": successSchema},
		}
	}
	out := map[string]any{
		success: resp,
		"default": map[string]any{
			"description": "application/problem+json domain error",
			"content": map[string]any{
				capabilities.ProblemContentType: map[string]any{
					"schema": map[string]any{"$ref": "#/components/schemas/Problem"},
				},
			},
		},
	}
	return out
}

func schemaRefOrObject(name string) map[string]any {
	if name == "" {
		return map[string]any{"type": "object"}
	}
	return map[string]any{"$ref": "#/components/schemas/" + sanitizeSchemaName(name)}
}

func sanitizeSchemaName(name string) string {
	name = strings.ReplaceAll(name, ".", "")
	return name
}

func openAPIComponents() map[string]any {
	schemas := map[string]any{
		"Problem": map[string]any{
			"type":     "object",
			"required": []any{"type", "title", "status", "code"},
			"properties": map[string]any{
				"type":             map[string]any{"type": "string"},
				"title":            map[string]any{"type": "string"},
				"status":           map[string]any{"type": "integer"},
				"detail":           map[string]any{"type": "string"},
				"instance":         map[string]any{"type": "string"},
				"code":             map[string]any{"type": "string"},
				"retryable":        map[string]any{"type": "boolean"},
				"fieldViolations":  map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/FieldViolation"}},
				"currentRevision":  map[string]any{"type": "string"},
				"expectedRevision": map[string]any{"type": "string"},
				"remediation":      map[string]any{"type": "string"},
			},
		},
		"FieldViolation": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"code":    map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
			},
		},
		"Health": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{"type": "string"},
			},
		},
		"Session": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"csrf":  map[string]any{"type": "string"},
				"actor": map[string]any{"$ref": "#/components/schemas/SessionActor"},
			},
		},
		"SessionActor": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":     map[string]any{"type": "string"},
				"class":  map[string]any{"type": "string"},
				"role":   map[string]any{"type": "string"},
				"scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"groups": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
		"State": specSchema(),
		"Spec":  specFieldsSchema(),
	}
	// SchemaRef names from the registry become components.
	for _, c := range capabilities.All() {
		for _, ref := range []*capabilities.SchemaRef{c.InputSchema, c.OutputSchema} {
			if ref == nil || ref.Name == "" {
				continue
			}
			key := sanitizeSchemaName(ref.Name)
			if _, ok := schemas[key]; ok {
				continue
			}
			schemas[key] = map[string]any{
				"type":        "object",
				"description": "Application type " + ref.Name + " (internal/app).",
			}
		}
	}
	return map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth": map[string]any{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "token",
				"description":  "Required for non-loopback peers. Loopback (127.0.0.1 / ::1) may omit the token (Q-AUTH / dev-loopback-unauth).",
			},
			"cookieAuth": map[string]any{
				"type":        "apiKey",
				"in":          "cookie",
				"name":        auth.CookieName,
				"description": "Browser session cookie. CSRF header X-LabDNS-CSRF is required on cookie-authenticated non-GET requests.",
			},
		},
		"schemas": schemas,
	}
}

func specSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "labdns.dev/v1alpha1 LabDNS document. Field list is model.State wrapping model.Spec.",
		"required":    []any{"apiVersion", "kind", "spec"},
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string", "const": model.APIVersionV1Alpha1},
			"kind":       map[string]any{"type": "string", "const": model.KindLabDNS},
			"metadata": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":   map[string]any{"type": "string"},
					"labels": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				},
			},
			"spec": map[string]any{"$ref": "#/components/schemas/Spec"},
		},
	}
}

func specFieldsSchema() map[string]any {
	// Frozen v1alpha1 Spec fields from internal/model.Spec.
	return map[string]any{
		"type":        "object",
		"description": "model.Spec v1alpha1. Normative JSON Schema: api/jsonschema/labdns.dev.v1alpha1.json",
		"properties": map[string]any{
			"listeners":     map[string]any{"type": "object"},
			"access":        map[string]any{"type": "object"},
			"defaults":      map[string]any{"type": "object"},
			"zones":         map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"forwarding":    map[string]any{"type": "object"},
			"cache":         map[string]any{"type": "object"},
			"chaos":         map[string]any{"type": "object"},
			"observability": map[string]any{"type": "object"},
			"ui":            map[string]any{"type": "object"},
			"management":    map[string]any{"type": "object"},
		},
	}
}

func marshalSorted(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := writeSorted(&buf, tree, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeSorted(buf *bytes.Buffer, v any, indent int) error {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
			writeIndent(buf, indent+1)
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteString(": ")
			if err := writeSorted(buf, x[k], indent+1); err != nil {
				return err
			}
		}
		if len(keys) > 0 {
			buf.WriteByte('\n')
			writeIndent(buf, indent)
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, el := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
			writeIndent(buf, indent+1)
			if err := writeSorted(buf, el, indent+1); err != nil {
				return err
			}
		}
		if len(x) > 0 {
			buf.WriteByte('\n')
			writeIndent(buf, indent)
		}
		buf.WriteByte(']')
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}

func writeIndent(buf *bytes.Buffer, n int) {
	for i := 0; i < n; i++ {
		buf.WriteString("  ")
	}
}
