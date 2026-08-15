package domainerr

// Code is a stable, transport-independent domain error code.
type Code string

const (
	CodeValidationFailed           Code = "validation_failed"
	CodeRevisionConflict           Code = "revision_conflict"
	CodeIdempotencyConflict        Code = "idempotency_conflict"
	CodeNotFound                   Code = "not_found"
	CodeMethodNotAllowed           Code = "method_not_allowed"
	CodeAlreadyExists              Code = "already_exists"
	CodeForbidden                  Code = "forbidden"
	CodeUnauthenticated            Code = "unauthenticated"
	CodeRateLimited                Code = "rate_limited"
	CodeProtectedObject            Code = "protected_object"
	CodeChaosDisabled              Code = "chaos_disabled"
	CodeChaosBudgetExceeded        Code = "chaos_budget_exceeded"
	CodePolicyExpired              Code = "policy_expired"
	CodeUnsupportedCapability      Code = "unsupported_capability"
	CodeUnsupportedProtocolVersion Code = "unsupported_protocol_version"
	CodeUpstreamUnavailable        Code = "upstream_unavailable"
	CodeResolutionFailed           Code = "resolution_failed"
	CodeInternalError              Code = "internal_error"
)

// catalog is the closed first-GA code list. Retryable is advisory per class.
var catalog = []struct {
	Code      Code
	Retryable bool
}{
	{CodeValidationFailed, false},
	{CodeRevisionConflict, true},
	{CodeIdempotencyConflict, false},
	{CodeNotFound, false},
	{CodeMethodNotAllowed, false},
	{CodeAlreadyExists, false},
	{CodeForbidden, false},
	{CodeUnauthenticated, false},
	{CodeRateLimited, true},
	{CodeProtectedObject, false},
	{CodeChaosDisabled, false},
	{CodeChaosBudgetExceeded, true},
	{CodePolicyExpired, false},
	{CodeUnsupportedCapability, false},
	{CodeUnsupportedProtocolVersion, false},
	{CodeUpstreamUnavailable, true},
	{CodeResolutionFailed, true},
	{CodeInternalError, true},
}

// Codes returns the stable catalog in documented order.
func Codes() []Code {
	out := make([]Code, len(catalog))
	for i, e := range catalog {
		out[i] = e.Code
	}
	return out
}

// Retryable reports the catalog default for code. Unknown codes are not retryable.
func Retryable(code Code) bool {
	for _, e := range catalog {
		if e.Code == code {
			return e.Retryable
		}
	}
	return false
}
