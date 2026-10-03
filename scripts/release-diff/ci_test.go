package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

type checkTransport func(*http.Request) (*http.Response, error)

func (f checkTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchChecksPreservesRunIdentity(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = checkTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("filter") != "latest" {
			t.Errorf("missing latest filter: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"check_runs":[{"id":2,"name":"unit","status":"completed","conclusion":"success","head_sha":"abc"},{"id":1,"name":"unit","status":"in_progress","head_sha":"abc"}]}`)), Header: make(http.Header)}, nil
	})
	runs, err := fetchGitHubChecks("fixture-token", "owner/repo", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].ID != 2 || runs[1].ID != 1 {
		t.Fatalf("run identity lost: %+v", runs)
	}
	if err := releasecontract.EvaluateChecks([]string{"unit"}, runs, "abc"); err != nil {
		t.Fatal(err)
	}
}
