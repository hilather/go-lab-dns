package releasecontract

import (
	"fmt"
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

func TestEvaluateChecksRejectsMissingSHA(t *testing.T) {
	if err := EvaluateChecks([]string{"unit"}, []CheckRun{{Name: "unit", Status: "completed", Conclusion: "success"}}, "abc"); err == nil {
		t.Fatal("missing SHA satisfied exact-commit gate")
	}
}

func TestEvaluateChecksPendingRerunCannotReuseOldSuccess(t *testing.T) {
	for _, status := range []string{"queued", "in_progress"} {
		for _, withIDs := range []bool{false, true} {
			t.Run(status+fmt.Sprint(withIDs), func(t *testing.T) {
				old := CheckRun{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: "abc", CompletedAt: time.Now()}
				pending := CheckRun{Name: "unit", Status: status, HeadSHA: "abc"}
				if withIDs {
					old.ID = 1
					pending.ID = 2
				}
				for _, runs := range [][]CheckRun{{old, pending}, {pending, old}} {
					if err := EvaluateChecks([]string{"unit"}, runs, "abc"); err == nil {
						t.Fatal("pending rerun reused old success")
					}
				}
			})
		}
	}
}

func TestEvaluateChecksLatestRunIdentityWins(t *testing.T) {
	runs := []CheckRun{
		{ID: 2, Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: "abc"},
		{ID: 1, Name: "unit", Status: "in_progress", HeadSHA: "abc"},
	}
	if err := EvaluateChecks([]string{"unit"}, runs, "abc"); err != nil {
		t.Fatal(err)
	}
}
