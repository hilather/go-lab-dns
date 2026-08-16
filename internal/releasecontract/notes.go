package releasecontract

import (
	"fmt"
	"strings"
)

// NotesReport is the result of validating a release-notes file.
type NotesReport struct {
	MissingHeadings    []string
	Placeholders       []string
	Unaccounted        []string
	UncheckedReviews   []string
	MissingReviewItems []string
}

// Error implements error when the notes are incomplete.
func (r NotesReport) Error() string {
	var b strings.Builder
	b.WriteString("incomplete release notes")
	writeList(&b, "missing headings", r.MissingHeadings)
	writeList(&b, "unfilled template placeholders", r.Placeholders)
	writeList(&b, "undocumented public-surface diffs", r.Unaccounted)
	writeList(&b, "unchecked review boxes for changed surfaces", r.UncheckedReviews)
	writeList(&b, "missing review checklist items", r.MissingReviewItems)
	return b.String()
}

// Incomplete reports whether any gate failed.
func (r NotesReport) Incomplete() bool {
	return len(r.MissingHeadings) > 0 ||
		len(r.Placeholders) > 0 ||
		len(r.Unaccounted) > 0 ||
		len(r.UncheckedReviews) > 0 ||
		len(r.MissingReviewItems) > 0
}

func writeList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString("\n  ")
	b.WriteString(label)
	b.WriteString(":\n    ")
	b.WriteString(strings.Join(items, "\n    "))
}

// ValidateNotes checks required headings, leftover template text, and that
// every changed surface is named in the notes (or its review box is checked).
func ValidateNotes(notes string, changed []Surface) NotesReport {
	var r NotesReport
	for _, h := range RequiredNoteHeadings() {
		if !hasHeading(notes, h) {
			r.MissingHeadings = append(r.MissingHeadings, h)
		}
	}
	lower := strings.ToLower(notes)
	for _, p := range TemplatePlaceholders() {
		if strings.Contains(notes, p) {
			r.Placeholders = append(r.Placeholders, p)
		}
	}
	for _, s := range PublicSurfaces() {
		if !checklistPresent(notes, s.ReviewToken) {
			r.MissingReviewItems = append(r.MissingReviewItems, s.ReviewToken)
		}
	}
	for _, s := range changed {
		if accounted(notes, lower, s) {
			if checklistUnchecked(notes, s.ReviewToken) {
				r.UncheckedReviews = append(r.UncheckedReviews, s.ID+" ("+s.ReviewToken+")")
			}
			continue
		}
		r.Unaccounted = append(r.Unaccounted, s.ID+" ("+s.Path+")")
	}
	return r
}

func hasHeading(notes, heading string) bool {
	for _, prefix := range []string{"## " + heading, "## " + heading + "\n"} {
		if strings.Contains(notes, prefix) {
			return true
		}
	}
	// Allow trailing punctuation differences and Windows newlines.
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "## "+heading {
			return true
		}
	}
	return false
}

func accounted(notes, lower string, s Surface) bool {
	if strings.Contains(lower, strings.ToLower(s.ID)) {
		return true
	}
	if strings.Contains(lower, strings.ToLower(s.Title)) {
		return true
	}
	if strings.Contains(notes, s.Path) {
		return true
	}
	return checklistChecked(notes, s.ReviewToken)
}

func checklistPresent(notes, token string) bool {
	for _, line := range strings.Split(notes, "\n") {
		if strings.Contains(line, token) && (strings.Contains(line, "- [") || strings.Contains(line, "- [x]") || strings.Contains(line, "- [X]") || strings.Contains(line, "- [ ]")) {
			return true
		}
	}
	return false
}

func checklistChecked(notes, token string) bool {
	for _, line := range strings.Split(notes, "\n") {
		if !strings.Contains(line, token) {
			continue
		}
		trim := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(trim, "- [x]") {
			return true
		}
	}
	return false
}

func checklistUnchecked(notes, token string) bool {
	for _, line := range strings.Split(notes, "\n") {
		if !strings.Contains(line, token) {
			continue
		}
		trim := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(trim, "- [ ]") {
			return true
		}
	}
	return false
}

// FormatNotesError is a stable wrapper used by CLI exit paths.
func FormatNotesError(r NotesReport) error {
	if !r.Incomplete() {
		return nil
	}
	return fmt.Errorf("%s", r.Error())
}
