package domainerr

import "errors"

// FieldViolation is one structured validation path.
type FieldViolation struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is the public domain error. It never includes secrets or stack traces.
type Error struct {
	Code            Code             `json:"code"`
	Message         string           `json:"message"`
	Retryable       bool             `json:"retryable"`
	FieldViolations []FieldViolation `json:"fieldViolations,omitempty"`
	CurrentRevision string           `json:"currentRevision,omitempty"`
	Remediation     string           `json:"remediation,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Message
}

// Is reports whether target is an *Error with the same Code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e != nil && t != nil && e.Code == t.Code
}

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// New returns an error for code with the catalog retryable default.
func New(code Code, message string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Retryable: Retryable(code),
	}
}

// WithRemediation sets a safe operator/agent hint.
func (e *Error) WithRemediation(s string) *Error {
	if e != nil {
		e.Remediation = s
	}
	return e
}

// WithRevision records the current content revision for conflict responses.
func (e *Error) WithRevision(rev string) *Error {
	if e != nil {
		e.CurrentRevision = rev
	}
	return e
}

// WithViolations appends field violations.
func (e *Error) WithViolations(v ...FieldViolation) *Error {
	if e != nil && len(v) > 0 {
		e.FieldViolations = append(e.FieldViolations, v...)
	}
	return e
}

func ValidationFailed(message string, violations ...FieldViolation) *Error {
	return New(CodeValidationFailed, message).WithViolations(violations...)
}

func RevisionConflict(message, currentRevision string) *Error {
	return New(CodeRevisionConflict, message).WithRevision(currentRevision)
}

func IdempotencyConflict(message string) *Error {
	return New(CodeIdempotencyConflict, message)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, message)
}

func AlreadyExists(message string) *Error {
	return New(CodeAlreadyExists, message)
}

func Forbidden(message string) *Error {
	return New(CodeForbidden, message)
}

func Unauthenticated(message string) *Error {
	return New(CodeUnauthenticated, message)
}

func RateLimited(message string) *Error {
	return New(CodeRateLimited, message)
}

func ProtectedObject(message string) *Error {
	return New(CodeProtectedObject, message)
}

func ChaosDisabled(message string) *Error {
	return New(CodeChaosDisabled, message)
}

func ChaosBudgetExceeded(message string) *Error {
	return New(CodeChaosBudgetExceeded, message)
}

func PolicyExpired(message string) *Error {
	return New(CodePolicyExpired, message)
}

func UnsupportedCapability(message string) *Error {
	return New(CodeUnsupportedCapability, message)
}

func UnsupportedProtocolVersion(message string) *Error {
	return New(CodeUnsupportedProtocolVersion, message)
}

func UpstreamUnavailable(message string) *Error {
	return New(CodeUpstreamUnavailable, message)
}

func ResolutionFailed(message string) *Error {
	return New(CodeResolutionFailed, message)
}

func Internal(message string) *Error {
	return New(CodeInternalError, message)
}
