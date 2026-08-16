// Command release-diff compares public surfaces between two git refs and
// optionally gates a tag on complete release notes and green required CI.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

func main() {
	var (
		notesPath     = flag.String("notes", "", "release notes file to validate against the diff")
		notesOnly     = flag.Bool("notes-only", false, "validate --notes headings only; do not compare refs")
		jsonOut       = flag.Bool("json", false, "print the report as JSON")
		allowDirty    = flag.Bool("allow-dirty", false, "permit a dirty worktree (tests only)")
		previousTag   = flag.Bool("previous-tag", false, "print the previous annotated/lightweight tag, or the empty tree")
		requireCI     = flag.Bool("require-ci", false, "require all required CI jobs success on HEAD")
		ciFixture     = flag.String("ci-fixture", "", "JSON file of check runs (tests / offline)")
		requiredJobs  = flag.String("required-jobs", strings.Join(releasecontract.RequiredCIJobs(), ","), "comma-separated required CI job names")
		emptyTreeFlag = flag.Bool("print-empty-tree", false, "print the git empty-tree SHA and exit")
	)
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}

	if *emptyTreeFlag {
		fmt.Println(emptyTreeSHA)
		return
	}
	if *previousTag {
		tag, err := previousTagName(root)
		if err != nil {
			fatal(err)
		}
		fmt.Println(tag)
		return
	}
	if *requireCI {
		sha, err := gitOutput(root, "rev-parse", "HEAD")
		if err != nil {
			fatal(err)
		}
		jobs := splitCSV(*requiredJobs)
		runs, err := loadCheckRuns(*ciFixture)
		if err != nil {
			fatal(err)
		}
		if err := releasecontract.EvaluateChecks(jobs, runs, strings.TrimSpace(sha)); err != nil {
			fatal(err)
		}
		return
	}
	if *notesOnly {
		if *notesPath == "" {
			fatal(fmt.Errorf("-notes-only requires -notes"))
		}
		body, err := os.ReadFile(*notesPath)
		if err != nil {
			fatal(fmt.Errorf("read notes: %w", err))
		}
		rep := releasecontract.ValidateNotes(string(body), nil)
		if err := releasecontract.FormatNotesError(rep); err != nil {
			fatal(err)
		}
		return
	}

	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: release-diff [flags] FROM TO\n")
		os.Exit(2)
	}
	if err := runDiff(root, args[0], args[1], *notesPath, *jsonOut, *allowDirty); err != nil {
		fatal(err)
	}
}

func runDiff(root, from, to, notesPath string, jsonOut, allowDirty bool) error {
	if !allowDirty {
		if err := rejectDirty(root); err != nil {
			return err
		}
	}
	from = resolveFrom(root, from)
	report, err := Compare(root, from, to)
	if err != nil {
		return err
	}
	if notesPath != "" {
		body, err := os.ReadFile(notesPath)
		if err != nil {
			return fmt.Errorf("read notes: %w", err)
		}
		changed := report.ChangedSurfaces()
		nrep := releasecontract.ValidateNotes(string(body), changed)
		report.Notes = &nrep
		if nrep.Incomplete() {
			if jsonOut {
				_ = json.NewEncoder(os.Stdout).Encode(report)
			} else {
				fmt.Fprint(os.Stderr, report.Text())
			}
			return releasecontract.FormatNotesError(nrep)
		}
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Print(report.Text())
	return nil
}

func resolveFrom(root, from string) string {
	if from == "" || from == "-" || from == "empty" || from == emptyTreeSHA {
		return emptyTreeSHA
	}
	return from
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "release-diff: %v\n", err)
	os.Exit(1)
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return repoRootFrom(wd)
}
