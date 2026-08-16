package releasecontract

import (
	"fmt"
	"strings"
)

// CheckRun is one GitHub check-run or workflow job result.
type CheckRun struct {
	Name       string
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, cancelled, skipped, ...
	HeadSHA    string
}

// EvaluateChecks fails if any required job is missing, not successful, or
// not recorded against wantSHA. A skipped or cancelled required job is a
// failed gate — there is no administrative bypass.
func EvaluateChecks(required []string, runs []CheckRun, wantSHA string) error {
	byName := map[string][]CheckRun{}
	for _, r := range runs {
		for _, key := range checkNameKeys(r.Name) {
			byName[key] = append(byName[key], r)
		}
	}
	var problems []string
	for _, name := range required {
		hits := byName[name]
		if len(hits) == 0 {
			problems = append(problems, name+": missing (required job did not run)")
			continue
		}
		best := latestCompleted(hits)
		if best == nil {
			problems = append(problems, name+": not completed (status="+hits[len(hits)-1].Status+")")
			continue
		}
		if wantSHA != "" && best.HeadSHA != "" && best.HeadSHA != wantSHA {
			problems = append(problems, fmt.Sprintf("%s: head SHA %s != tag commit %s", name, best.HeadSHA, wantSHA))
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

func latestCompleted(runs []CheckRun) *CheckRun {
	var last *CheckRun
	for i := range runs {
		r := &runs[i]
		if strings.EqualFold(r.Status, "completed") {
			last = r
		}
	}
	return last
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
