package config

import (
	"path"
	"strings"
)

// ValidManagementMountPath reports whether a literal HTTP path can be mounted
// without colliding with REST/UI namespaces or introducing ServeMux syntax.
func ValidManagementMountPath(value string) bool {
	if value == "" || value == "/" || value == "/v1" || strings.HasPrefix(value, "/v1/") || !strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	first := strings.Split(strings.TrimPrefix(value, "/"), "/")[0]
	switch first {
	case "v1", "login", "state", "changes", "zones", "resolve", "forwarding", "cache", "chaos", "audit", "schema", "docs", "capabilities", "reset", "assets", "index.html", "favicon.ico":
		return false
	}
	for _, r := range value {
		if r == '/' || r == '-' || r == '_' || r == '.' || r == '~' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}
