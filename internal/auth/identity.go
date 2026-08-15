package auth

// Actor is the authenticated caller shared by REST, MCP, and local signals.
type Actor struct {
	ID     string
	Class  string // token | mtls | proxy | local-signal | startup
	Scopes []string
	Groups []string
}
