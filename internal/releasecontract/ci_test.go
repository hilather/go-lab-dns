package releasecontract

import (
	"strings"
	"testing"
	"time"
)

func TestEvaluateChecksRequiresSuccessOnExactSHA(t *testing.T) {
	sha := "abc123"
	required := []string{"unit", "generated-file"}
	ok := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: sha},
		{Name: "generated-file", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	if err := EvaluateChecks(required, ok, sha); err != nil {
		t.Fatal(err)
	}

	failed := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "failure", HeadSHA: sha},
		{Name: "generated-file", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	err := EvaluateChecks(required, failed, sha)
	if err == nil || !strings.Contains(err.Error(), "unit") {
		t.Fatalf("want unit failure, got %v", err)
	}

	skipped := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "skipped", HeadSHA: sha},
		{Name: "generated-file", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	if err := EvaluateChecks(required, skipped, sha); err == nil {
		t.Fatal("skipped required job must fail the gate")
	}

	missing := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	if err := EvaluateChecks(required, missing, sha); err == nil || !strings.Contains(err.Error(), "generated-file") {
		t.Fatalf("want missing generated-file, got %v", err)
	}

	wrongSHA := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: "other"},
		{Name: "generated-file", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	if err := EvaluateChecks(required, wrongSHA, sha); err == nil || !strings.Contains(err.Error(), "head SHA") {
		t.Fatalf("want SHA mismatch, got %v", err)
	}

	// A success on another SHA must not satisfy the tag commit.
	staleSuccess := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: "other"},
		{Name: "generated-file", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	if err := EvaluateChecks(required, staleSuccess, sha); err == nil || !strings.Contains(err.Error(), "unit") {
		t.Fatalf("want unit SHA mismatch, got %v", err)
	}
}

func TestEvaluateChecksAcceptsWorkflowPrefixedNames(t *testing.T) {
	sha := "abc123"
	runs := []CheckRun{
		{Name: "CI / unit", Status: "completed", Conclusion: "success", HeadSHA: sha},
		{Name: "CI / generated-file", Status: "completed", Conclusion: "success", HeadSHA: sha},
	}
	if err := EvaluateChecks([]string{"unit", "generated-file"}, runs, sha); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateChecksUsesNewestCompletedAt(t *testing.T) {
	sha := "abc"
	older := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Minute)

	// Newer failure must win even when it appears first in the slice.
	newerFail := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "failure", HeadSHA: sha, CompletedAt: newer},
		{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: sha, CompletedAt: older},
	}
	if err := EvaluateChecks([]string{"unit"}, newerFail, sha); err == nil {
		t.Fatal("newer failure must fail the gate")
	}

	// Newer success must win even when an older failure is last in the slice.
	newerOK := []CheckRun{
		{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: sha, CompletedAt: newer},
		{Name: "unit", Status: "completed", Conclusion: "failure", HeadSHA: sha, CompletedAt: older},
	}
	if err := EvaluateChecks([]string{"unit"}, newerOK, sha); err != nil {
		t.Fatal(err)
	}
}

func TestRequiredCIJobsHaveNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, j := range RequiredCIJobs() {
		if seen[j] {
			t.Fatalf("duplicate job %q", j)
		}
		seen[j] = true
	}
}
