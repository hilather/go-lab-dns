package capabilities

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func TestEveryCatalogCodeHasMapping(t *testing.T) {
	if len(errorMap) != len(domainerr.Codes()) {
		t.Fatalf("errorMap=%d catalog=%d", len(errorMap), len(domainerr.Codes()))
	}
	seenStatus := map[int]bool{}
	seenRPC := map[int]bool{}
	for _, code := range domainerr.Codes() {
		m, ok := errorMap[code]
		if !ok {
			t.Errorf("no mapping for %s", code)
			continue
		}
		if m.Status < 400 || m.Status > 599 {
			t.Errorf("%s status=%d", code, m.Status)
		}
		if m.Title == "" {
			t.Errorf("%s empty title", code)
		}
		if HTTPStatus(code) != m.Status || JSONRPCCode(code) != m.RPC || TitleFor(code) != m.Title {
			t.Errorf("%s helper mismatch", code)
		}
		seenStatus[m.Status] = true
		seenRPC[m.RPC] = true
		urn := ProblemTypeURN(code)
		if !strings.HasPrefix(urn, ProblemTypePrefix) || strings.Contains(urn, "_") {
			t.Errorf("type URN %q", urn)
		}
	}
	if !seenStatus[409] || !seenRPC[JSONRPCConflict] {
		t.Fatal("expected conflict mapping")
	}
}

func TestErrorMapMatchesDocs17Tables(t *testing.T) {
	httpPairs := parseDocs17CodeIntTable(t, "Domain code | HTTP status")
	rpcPairs := parseDocs17CodeIntTable(t, "Domain code | JSON-RPC code")
	codes := domainerr.Codes()
	if len(httpPairs) != len(codes) {
		t.Fatalf("docs/17 HTTP table has %d rows, catalog has %d", len(httpPairs), len(codes))
	}
	if len(rpcPairs) != len(codes) {
		t.Fatalf("docs/17 JSON-RPC table has %d rows, catalog has %d", len(rpcPairs), len(codes))
	}
	for _, code := range codes {
		wantHTTP, ok := httpPairs[code]
		if !ok {
			t.Errorf("docs/17 HTTP table missing %s", code)
			continue
		}
		wantRPC, ok := rpcPairs[code]
		if !ok {
			t.Errorf("docs/17 JSON-RPC table missing %s", code)
			continue
		}
		if HTTPStatus(code) != wantHTTP {
			t.Errorf("%s HTTPStatus=%d docs/17=%d", code, HTTPStatus(code), wantHTTP)
		}
		if JSONRPCCode(code) != wantRPC {
			t.Errorf("%s JSONRPCCode=%d docs/17=%d", code, JSONRPCCode(code), wantRPC)
		}
	}
}

func parseDocs17CodeIntTable(t *testing.T, header string) map[domainerr.Code]int {
	t.Helper()
	body := readRepoFile(t, "docs", "17-error-model.md")
	idx := strings.Index(body, header)
	if idx < 0 {
		t.Fatalf("docs/17-error-model.md: missing table %q", header)
	}
	out := make(map[domainerr.Code]int)
	for _, line := range strings.Split(body[idx:], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if len(out) > 0 {
				break
			}
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 2 || cells[0] == "Domain code" || strings.HasPrefix(cells[0], "---") {
			continue
		}
		codes := backticks(cells[0])
		if len(codes) != 1 {
			t.Fatalf("docs/17 row %q: want one domain code", line)
		}
		n, err := strconv.Atoi(strings.TrimSpace(cells[1]))
		if err != nil {
			t.Fatalf("docs/17 row %q: %v", line, err)
		}
		code := domainerr.Code(codes[0])
		if _, dup := out[code]; dup {
			t.Fatalf("docs/17 duplicate code %s", code)
		}
		out[code] = n
	}
	if len(out) == 0 {
		t.Fatalf("parsed zero rows from docs/17 table %q", header)
	}
	return out
}

func TestProblemAndJSONRPCShareDomainData(t *testing.T) {
	err := domainerr.RevisionConflict("The active state changed after the plan was created.", "sha256:abc").
		WithRemediation("Re-read and re-plan.")
	p := ProblemFrom(err, "urn:labdns:request:01JTEST")
	rpc := JSONRPCFrom(err)
	if p.Code != rpc.Data.Code || p.Code != domainerr.CodeRevisionConflict {
		t.Fatalf("code rest=%s mcp=%s", p.Code, rpc.Data.Code)
	}
	if p.Retryable != rpc.Data.Retryable || !p.Retryable {
		t.Fatalf("retryable rest=%v mcp=%v", p.Retryable, rpc.Data.Retryable)
	}
	if p.CurrentRevision != rpc.Data.CurrentRevision || p.CurrentRevision != "sha256:abc" {
		t.Fatalf("revision rest=%q mcp=%q", p.CurrentRevision, rpc.Data.CurrentRevision)
	}
	if p.Status != 409 || rpc.Code != JSONRPCConflict {
		t.Fatalf("transport codes rest=%d mcp=%d (must differ by transport, these are the frozen hints)", p.Status, rpc.Code)
	}
	if p.Type != "urn:labdns:error:revision-conflict" {
		t.Fatalf("type=%q", p.Type)
	}
	if p.Title != "State revision conflict" {
		t.Fatalf("title=%q", p.Title)
	}
	if p.Instance != "urn:labdns:request:01JTEST" {
		t.Fatalf("instance=%q", p.Instance)
	}
	if p.Detail != err.Message || rpc.Message != err.Message {
		t.Fatalf("detail/message mismatch %q %q", p.Detail, rpc.Message)
	}

	raw, jerr := json.Marshal(p)
	if jerr != nil {
		t.Fatal(jerr)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"type", "title", "status", "detail", "instance", "code", "retryable", "currentRevision", "remediation"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("problem missing %s in %s", k, raw)
		}
	}
	if obj["code"] != string(domainerr.CodeRevisionConflict) {
		t.Fatalf("code field=%v", obj["code"])
	}
}

func TestValidationProblemCopiesViolations(t *testing.T) {
	v := domainerr.FieldViolation{Path: "spec.zones[0]", Code: "dup", Message: "duplicate"}
	err := domainerr.ValidationFailed("Candidate state is invalid.", v)
	p := ProblemFrom(err, "")
	rpc := JSONRPCFrom(err)
	if p.Status != 400 || rpc.Code != JSONRPCInvalidParams {
		t.Fatalf("validation rest=%d mcp=%d", p.Status, rpc.Code)
	}
	if len(p.FieldViolations) != 1 || p.FieldViolations[0].Path != v.Path {
		t.Fatalf("problem violations=%+v", p.FieldViolations)
	}
	p.FieldViolations[0].Path = "mutated"
	if rpc.Data.FieldViolations[0].Path != v.Path {
		t.Fatal("shared violation slice across transports")
	}
}

func TestUnknownErrorDoesNotLeak(t *testing.T) {
	err := errors.New("secret token=abc stack goroutine 1 [running]:")
	p := ProblemFrom(err, "")
	rpc := JSONRPCFrom(err)
	if p.Code != domainerr.CodeInternalError || rpc.Data.Code != domainerr.CodeInternalError {
		t.Fatalf("want internal, rest=%s mcp=%s", p.Code, rpc.Data.Code)
	}
	if p.Status != 500 || rpc.Code != JSONRPCInternalError {
		t.Fatalf("internal transport rest=%d mcp=%d", p.Status, rpc.Code)
	}
	if p.Detail != "internal error" || rpc.Data.Message != "internal error" {
		t.Fatalf("leaked %q / %q", p.Detail, rpc.Data.Message)
	}
	if strings.Contains(p.Detail, "secret") || strings.Contains(rpc.Message, "token=") {
		t.Fatal("unknown error text leaked")
	}
}

func TestNilAndEmptyMessage(t *testing.T) {
	p := ProblemFrom(nil, "")
	if p.Code != domainerr.CodeInternalError || p.Status != 500 {
		t.Fatalf("nil err = %+v", p)
	}
	rpc := JSONRPCFrom(&domainerr.Error{Code: domainerr.CodeNotFound})
	if rpc.Code != JSONRPCNotFound || rpc.Message == "" {
		t.Fatalf("empty message rpc=%+v", rpc)
	}
}

func TestUnknownCodeHelpers(t *testing.T) {
	code := domainerr.Code("not_a_real_code")
	if HTTPStatus(code) != 500 || JSONRPCCode(code) != JSONRPCInternalError {
		t.Fatal("unknown code should map to internal")
	}
	if ProblemTypeURN(code) != "urn:labdns:error:not-a-real-code" {
		t.Fatalf("urn=%q", ProblemTypeURN(code))
	}
}
