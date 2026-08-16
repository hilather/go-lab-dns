package releasecontract

import (
	"fmt"
	"strings"
	"time"
)

// CheckRun is one GitHub check-run or workflow job result.
type CheckRun struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	HeadSHA     string    `json:"headSha"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

// EvaluateChecks fails if any required job is missing, not successful, or
// not recorded against wantSHA. A skipped or cancelled required job is a
// failed gate — there is no administrative bypass.
func EvaluateChecks(required []string, runs []CheckRun, wantSHA string) error {
	var problems []string
	for _, name := range required {
		hits, wrongSHA := matchingRuns(runs, name, wantSHA)
		if len(hits) == 0 {
			if wrongSHA > 0 {
				problems = append(problems, fmt.Sprintf("%s: head SHA != tag commit %s", name, wantSHA))
			} else {
				problems = append(problems, name+": missing (required job did not run)")
			}
			continue
		}
		best := latestCompleted(hits)
		if best == nil {
			problems = append(problems, name+": not completed (status="+hits[len(hits)-1].Status+")")
			continue
		}
		if !strings.EqualFold(best.Status, "completed") || !strings.EqualFold(best.Conclusion, "success") {
			problems = append(problems, fmt.Sprintf("%s: %s/%s (required job must succeed)", name, best.Status, best.Conclusion))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("release cannot proceed after a failed or missing required job:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func matchingRuns(runs []CheckRun, name, wantSHA string) (hits []CheckRun, wrongSHA int) {
	for _, r := range runs {
		if !checkNameMatches(r.Name, name) {
			continue
		}
		if wantSHA != "" && r.HeadSHA != "" && r.HeadSHA != wantSHA {
			wrongSHA++
			continue
		}
		hits = append(hits, r)
	}
	return hits, wrongSHA
}

func latestCompleted(runs []CheckRun) *CheckRun {
	var best *CheckRun
	for i := range runs {
		r := &runs[i]
		if !strings.EqualFold(r.Status, "completed") {
			continue
		}
		if best == nil {
			best = r
			continue
		}
		switch {
		case r.CompletedAt.After(best.CompletedAt):
			best = r
		case r.CompletedAt.Equal(best.CompletedAt):
			// Equal or both zero: later in the filtered slice wins.
			best = r
		}
	}
	return best
}

func checkNameMatches(got, want string) bool {
	for _, key := range checkNameKeys(got) {
		if key == want {
			return true
		}
	}
	return false
}

// checkNameKeys indexes a check-run by its API name and the suffix after
// " / " so "CI / format" still satisfies required job "format".
func checkNameKeys(name string) []string {
	name = strings.TrimSpace(name)
	keys := []string{name}
	if i := strings.LastIndex(name, " / "); i >= 0 {
		if suffix := strings.TrimSpace(name[i+3:]); suffix != "" && suffix != name {
			keys = append(keys, suffix)
		}
	}
	return keys
}
