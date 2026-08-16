package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"os"
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Token is one bearer binding. Role expands to scopes when Scopes is empty.
type Token struct {
	Token  string   `json:"token"`
	ID     string   `json:"id,omitempty"`
	Role   string   `json:"role,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
	Groups []string `json:"groups,omitempty"`
}

// Policy is the first-GA management authenticator.
type Policy struct {
	profile string
	tokens  []Token
}

// PolicyConfig constructs a Policy.
type PolicyConfig struct {
	// Profile is dev-loopback-unauth or bearer. Empty is dev-loopback-unauth.
	Profile string
	// SecretRef is a file of tokens. Required when Profile is bearer.
	SecretRef string
	// Tokens are in-process bindings. Merged with SecretRef when both are set.
	Tokens []Token
}

// NewPolicy loads tokens and validates the profile.
func NewPolicy(cfg PolicyConfig) (*Policy, error) {
	profile := cfg.Profile
	if profile == "" {
		profile = ProfileDevLoopbackUnauth
	}
	switch profile {
	case ProfileDevLoopbackUnauth, ProfileBearer:
	default:
		return nil, domainerr.ValidationFailed("unknown auth profile",
			domainerr.FieldViolation{Path: "spec.management.auth.profile", Code: "invalid_value", Message: "profile must be dev-loopback-unauth or bearer"})
	}
	toks := append([]Token(nil), cfg.Tokens...)
	if ref := strings.TrimSpace(cfg.SecretRef); ref != "" {
		loaded, err := LoadTokens(ref)
		if err != nil {
			return nil, err
		}
		toks = append(toks, loaded...)
	}
	if profile == ProfileBearer && len(toks) == 0 {
		return nil, domainerr.ValidationFailed("bearer profile requires tokens",
			domainerr.FieldViolation{Path: "spec.management.auth.secretRef", Code: "required", Message: "bearer profile requires at least one token"})
	}
	for i := range toks {
		if strings.TrimSpace(toks[i].Token) == "" {
			return nil, domainerr.ValidationFailed("empty token",
				domainerr.FieldViolation{Path: "tokens", Code: "required", Message: "token value is required"})
		}
		if toks[i].Role != "" && !KnownRole(toks[i].Role) && len(toks[i].Scopes) == 0 {
			return nil, domainerr.ValidationFailed("unknown role",
				domainerr.FieldViolation{Path: "tokens.role", Code: "invalid_value", Message: "unknown role " + toks[i].Role})
		}
	}
	return &Policy{profile: profile, tokens: toks}, nil
}

// Profile is the configured auth profile.
func (p *Policy) Profile() string {
	if p == nil {
		return ProfileDevLoopbackUnauth
	}
	return p.profile
}

// Authenticate looks up token. Unknown tokens fail closed.
func (p *Policy) Authenticate(ctx context.Context, token string) (Actor, error) {
	_ = ctx
	if p == nil {
		return Actor{}, domainerr.Unauthenticated("authentication required")
	}
	if strings.TrimSpace(token) == "" {
		return Actor{}, domainerr.Unauthenticated("authentication required")
	}
	var found *Token
	for i := range p.tokens {
		if tokenEq(p.tokens[i].Token, token) {
			// Keep scanning so compare cost does not advertise index.
			if found == nil {
				t := p.tokens[i]
				found = &t
			}
		}
	}
	if found == nil {
		return Actor{}, domainerr.Unauthenticated("invalid token")
	}
	id := found.ID
	if id == "" {
		id = "bearer"
	}
	role := found.Role
	scopes := append([]string(nil), found.Scopes...)
	if len(scopes) == 0 && role == "" {
		role = RoleAdministrator
	}
	return Actor{
		ID:     id,
		Class:  ClassToken,
		Role:   role,
		Scopes: scopes,
		Groups: append([]string(nil), found.Groups...),
	}, nil
}

func tokenEq(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}

// LoadTokens reads secretRef. A file may be:
//   - a single token (administrator)
//   - JSON {"tokens":[...]} or a JSON array of Token
func LoadTokens(path string) ([]Token, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, domainerr.Unauthenticated("token secret is unavailable")
	}
	return ParseTokens(b)
}

// ParseTokens decodes a secret file body.
func ParseTokens(raw []byte) ([]Token, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, domainerr.ValidationFailed("empty token secret",
			domainerr.FieldViolation{Path: "secretRef", Code: "required", Message: "token secret is empty"})
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		var wrap struct {
			Tokens []Token `json:"tokens"`
		}
		if err := json.Unmarshal([]byte(s), &wrap); err == nil && len(wrap.Tokens) > 0 {
			return wrap.Tokens, nil
		}
		var arr []Token
		if err := json.Unmarshal([]byte(s), &arr); err == nil && len(arr) > 0 {
			return arr, nil
		}
		return nil, domainerr.ValidationFailed("invalid token secret",
			domainerr.FieldViolation{Path: "secretRef", Code: "invalid_value", Message: "token secret JSON is invalid"})
	}
	// First non-comment line is the token; remaining lines are ignored.
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return []Token{{Token: line, Role: RoleAdministrator}}, nil
	}
	return nil, domainerr.ValidationFailed("empty token secret",
		domainerr.FieldViolation{Path: "secretRef", Code: "required", Message: "token secret is empty"})
}

// FromSpec builds a Policy from canonical management auth.
func FromSpec(spec model.AuthSpec) (*Policy, error) {
	return NewPolicy(PolicyConfig{Profile: spec.Profile, SecretRef: spec.SecretRef})
}
