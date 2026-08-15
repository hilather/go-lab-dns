package domainerr

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var documentedCodes = []Code{
	"validation_failed",
	"revision_conflict",
	"idempotency_conflict",
	"not_found",
	"already_exists",
	"forbidden",
	"unauthenticated",
	"rate_limited",
	"protected_object",
	"chaos_disabled",
	"chaos_budget_exceeded",
	"policy_expired",
	"unsupported_capability",
	"unsupported_protocol_version",
	"upstream_unavailable",
	"resolution_failed",
	"internal_error",
}

func TestCatalogMatchesDocsAndConstants(t *testing.T) {
	got := Codes()
	if len(got) != len(documentedCodes) {
		t.Fatalf("catalog len=%d, docs len=%d (half-migration?)\n catalog=%v\n docs=%v", len(got), len(documentedCodes), got, documentedCodes)
	}
	seen := map[Code]bool{}
	for i, code := range got {
		if code != documentedCodes[i] {
			t.Fatalf("catalog[%d]=%q, docs=%q", i, code, documentedCodes[i])
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}

	fromDocs := codesFromErrorModelDoc(t)
	if len(fromDocs) != len(documentedCodes) {
		t.Fatalf("docs/17 parsed %d codes, catalog has %d: parsed=%v", len(fromDocs), len(documentedCodes), fromDocs)
	}
	for i, code := range documentedCodes {
		if fromDocs[i] != code {
			t.Fatalf("docs/17[%d]=%q, catalog=%q", i, fromDocs[i], code)
		}
	}

	consts := codeConstants(t)
	if len(consts) != len(got) {
		t.Fatalf("Code constants=%v catalog=%v", consts, got)
	}
	for _, code := range got {
		if !consts[string(code)] {
			t.Fatalf("catalog code %q has no Code constant", code)
		}
	}
}

func TestEveryCodeHasConstructorAndRetryableDefault(t *testing.T) {
	ctors := map[Code]func(string) *Error{
		CodeValidationFailed:           func(m string) *Error { return ValidationFailed(m) },
		CodeRevisionConflict:           func(m string) *Error { return RevisionConflict(m, "sha256:abc") },
		CodeIdempotencyConflict:        IdempotencyConflict,
		CodeNotFound:                   NotFound,
		CodeAlreadyExists:              AlreadyExists,
		CodeForbidden:                  Forbidden,
		CodeUnauthenticated:            Unauthenticated,
		CodeRateLimited:                RateLimited,
		CodeProtectedObject:            ProtectedObject,
		CodeChaosDisabled:              ChaosDisabled,
		CodeChaosBudgetExceeded:        ChaosBudgetExceeded,
		CodePolicyExpired:              PolicyExpired,
		CodeUnsupportedCapability:      UnsupportedCapability,
		CodeUnsupportedProtocolVersion: UnsupportedProtocolVersion,
		CodeUpstreamUnavailable:        UpstreamUnavailable,
		CodeResolutionFailed:           ResolutionFailed,
		CodeInternalError:              Internal,
	}
	if len(ctors) != len(catalog) {
		t.Fatalf("constructor map len=%d catalog=%d", len(ctors), len(catalog))
	}
	for _, e := range catalog {
		fn, ok := ctors[e.Code]
		if !ok {
			t.Fatalf("no constructor for %q", e.Code)
		}
		err := fn("msg")
		if err.Code != e.Code {
			t.Fatalf("%s constructor code=%q", e.Code, err.Code)
		}
		if err.Message != "msg" {
			t.Fatalf("%s message=%q", e.Code, err.Message)
		}
		if err.Retryable != e.Retryable {
			t.Fatalf("%s retryable=%v, want %v", e.Code, err.Retryable, e.Retryable)
		}
		if err.Retryable != Retryable(e.Code) {
			t.Fatalf("Retryable(%s) mismatch", e.Code)
		}
	}
}

func TestErrorJSONShapeAndNoStack(t *testing.T) {
	err := ValidationFailed("Candidate state is invalid.", FieldViolation{
		Path:    "spec.chaos.policies[0].outcomes[0]",
		Code:    "conflicting_transport_actions",
		Message: "drop and tcp-reset cannot be selected in one outcome",
	}).WithRemediation("Split the transport actions into exclusive outcomes.").WithRevision("sha256:deadbeef")

	raw, jerr := json.Marshal(err)
	if jerr != nil {
		t.Fatalf("marshal: %v", jerr)
	}
	var obj map[string]any
	if jerr := json.Unmarshal(raw, &obj); jerr != nil {
		t.Fatalf("unmarshal: %v", jerr)
	}
	for _, k := range []string{"code", "message", "retryable", "fieldViolations", "currentRevision", "remediation"} {
		if _, ok := obj[k]; !ok {
			t.Fatalf("missing JSON key %q in %s", k, raw)
		}
	}
	if obj["code"] != string(CodeValidationFailed) {
		t.Fatalf("code=%v", obj["code"])
	}
	if obj["retryable"] != false {
		t.Fatalf("retryable=%v", obj["retryable"])
	}

	s := err.Error()
	if strings.Contains(s, "goroutine") || strings.Contains(s, ".go:") || strings.Contains(s, "\n") {
		t.Fatalf("Error() looks like a stack trace: %q", s)
	}
	if strings.Contains(s, "SECRET") {
		t.Fatalf("Error() leaked unexpected text: %q", s)
	}
}

func TestAsAndIs(t *testing.T) {
	base := NotFound("zone")
	wrapped := errors.Join(base, errors.New("wrap"))
	got, ok := As(wrapped)
	if !ok || got.Code != CodeNotFound {
		t.Fatalf("As = (%v, %v)", got, ok)
	}
	if !errors.Is(wrapped, &Error{Code: CodeNotFound}) {
		t.Fatal("errors.Is did not match by code")
	}
	if errors.Is(wrapped, &Error{Code: CodeForbidden}) {
		t.Fatal("errors.Is matched a different code")
	}
}

func TestUnknownCodeNotRetryable(t *testing.T) {
	if Retryable(Code("not_a_real_code")) {
		t.Fatal("unknown code should not be retryable")
	}
	err := New(Code("not_a_real_code"), "x")
	if err.Retryable {
		t.Fatal("New(unknown) retryable")
	}
}

func TestNoRuntimeDebugImport(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == "runtime/debug" || path == "runtime" {
					t.Errorf("%s imports %q (stack traces must not be captured)", name, path)
				}
			}
		}
	}
}

func codesFromErrorModelDoc(t *testing.T) []Code {
	t.Helper()
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "docs", "17-error-model.md"))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile("(?s)Stable codes include:\n\n```text\n(.+?)\n```")
	m := re.FindSubmatch(body)
	if m == nil {
		t.Fatal("docs/17-error-model.md: could not find 'Stable codes include' fence")
	}
	var out []Code
	for _, line := range strings.Split(string(m[1]), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, Code(line))
	}
	return out
}

func codeConstants(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.Name, "_test") {
			continue
		}
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				gen, ok := d.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs := spec.(*ast.ValueSpec)
					if vs.Type == nil {
						continue
					}
					id, ok := vs.Type.(*ast.Ident)
					if !ok || id.Name != "Code" {
						continue
					}
					for _, v := range vs.Values {
						lit, ok := v.(*ast.BasicLit)
						if !ok || lit.Kind != token.STRING {
							continue
						}
						s, err := strconv.Unquote(lit.Value)
						if err != nil {
							t.Fatal(err)
						}
						out[s] = true
					}
				}
			}
		}
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestNilError(t *testing.T) {
	var err *Error
	if err.Error() != "" {
		t.Fatalf("nil Error()=%q", err.Error())
	}
	if err.WithRemediation("x") != nil || err.WithRevision("x") != nil || err.WithViolations(FieldViolation{}) != nil {
		t.Fatal("nil receiver With* should return nil")
	}
}
