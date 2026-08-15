package capabilities

// SchemaRef names a future OpenAPI/JSON Schema component. Adapters generate
// contracts from these names in API-001/MCP-001; this PR freezes the name only.
type SchemaRef struct {
	Name string `json:"name,omitempty"`
}

// RESTBinding is one HTTP method+path. Path templates are frozen spellings.
type RESTBinding struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// MCPBinding is the tool and optional resource surface for one capability.
// Tools is empty only for REST-only health probes.
type MCPBinding struct {
	Tools     []string `json:"tools,omitempty"`
	Resources []string `json:"resources,omitempty"`
	// DocsID parameterizes the shared dns_docs_get tool.
	DocsID string `json:"docsId,omitempty"`
}

// Capability is one frozen row of the REST↔MCP table.
//
// Handler funcs are not stored here so this package does not import
// internal/app (app discovery reads the registry). Adapters bind
// ServiceMethods by name in later PRs.
type Capability struct {
	ID             ID            `json:"id"`
	Title          string        `json:"title"`
	Version        string        `json:"version"`
	Description    string        `json:"description"`
	InputSchema    *SchemaRef    `json:"inputSchema,omitempty"`
	OutputSchema   *SchemaRef    `json:"outputSchema,omitempty"`
	RequiredScopes []string      `json:"requiredScopes,omitempty"`
	Mutating       bool          `json:"mutating"`
	Idempotent     bool          `json:"idempotent"`
	RESTOnly       bool          `json:"restOnly,omitempty"`
	REST           []RESTBinding `json:"rest"`
	MCP            *MCPBinding   `json:"mcp"`
	ServiceMethods []string      `json:"serviceMethods,omitempty"`
}

// RESTRef is Method plus Path, used as a lookup key ("GET /v1/state").
func (b RESTBinding) RESTRef() string {
	if b.Method == "" {
		return b.Path
	}
	return b.Method + " " + b.Path
}
