package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

// Status is the per-surface comparison result.
type Status string

const (
	StatusIdentical Status = "identical"
	StatusChanged   Status = "changed"
	StatusAdded     Status = "added"
	StatusRemoved   Status = "removed"
	StatusMissing   Status = "missing-both"
)

// SurfaceDelta is one compared public surface.
type SurfaceDelta struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Title      string `json:"title"`
	Status     Status `json:"status"`
	FromSHA256 string `json:"fromSha256,omitempty"`
	ToSHA256   string `json:"toSha256,omitempty"`
}

// Report is the machine-readable release-diff output.
type Report struct {
	From     string                       `json:"from"`
	To       string                       `json:"to"`
	Surfaces []SurfaceDelta               `json:"surfaces"`
	Notes    *releasecontract.NotesReport `json:"notes,omitempty"`
}

// Compare reads every public surface from two git refs.
func Compare(root, from, to string) (Report, error) {
	rep := Report{From: from, To: to}
	if err := verifyRef(root, from); err != nil {
		return rep, err
	}
	if err := verifyRef(root, to); err != nil {
		return rep, err
	}
	for _, s := range releasecontract.PublicSurfaces() {
		old, oldOK, err := fileAt(root, from, s.Path)
		if err != nil {
			return rep, fmt.Errorf("%s at %s: %w", s.Path, from, err)
		}
		neu, neuOK, err := fileAt(root, to, s.Path)
		if err != nil {
			return rep, fmt.Errorf("%s at %s: %w", s.Path, to, err)
		}
		d := SurfaceDelta{ID: s.ID, Path: s.Path, Title: s.Title}
		if oldOK {
			d.FromSHA256 = shaHex(old)
		}
		if neuOK {
			d.ToSHA256 = shaHex(neu)
		}
		switch {
		case !oldOK && !neuOK:
			d.Status = StatusMissing
		case !oldOK && neuOK:
			d.Status = StatusAdded
		case oldOK && !neuOK:
			d.Status = StatusRemoved
		case d.FromSHA256 == d.ToSHA256:
			d.Status = StatusIdentical
		default:
			d.Status = StatusChanged
		}
		rep.Surfaces = append(rep.Surfaces, d)
	}
	return rep, nil
}

// ChangedSurfaces returns public surfaces that are not identical.
func (r Report) ChangedSurfaces() []releasecontract.Surface {
	byID := map[string]releasecontract.Surface{}
	for _, s := range releasecontract.PublicSurfaces() {
		byID[s.ID] = s
	}
	var out []releasecontract.Surface
	for _, d := range r.Surfaces {
		if d.Status == StatusIdentical || d.Status == StatusMissing {
			continue
		}
		if s, ok := byID[d.ID]; ok {
			out = append(out, s)
		}
	}
	return out
}

// Text is the operator-readable report.
func (r Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "release-diff %s → %s\n", r.From, r.To)
	for _, d := range r.Surfaces {
		fmt.Fprintf(&b, "  %-16s %-10s %s\n", d.ID, d.Status, d.Path)
	}
	changed := r.ChangedSurfaces()
	if len(changed) == 0 {
		b.WriteString("no public-surface differences\n")
	} else {
		fmt.Fprintf(&b, "%d public-surface difference(s) require curated release notes\n", len(changed))
	}
	return b.String()
}

func shaHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
