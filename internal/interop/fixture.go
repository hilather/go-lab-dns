package interop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

// Case is one row of testdata/interop/cases.json.
type Case struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	WantRcode   string   `json:"wantRcode"`
	WantAA      bool     `json:"wantAA"`
	WantSOA     bool     `json:"wantSOA"`
	WantTTL     int      `json:"wantTTL"`
	WantCNAME   string   `json:"wantCNAME"`
	WantEDECode int      `json:"wantEDECode"`
	WantEDEText string   `json:"wantEDEText"`
	WantTC      bool     `json:"wantTC"`
	WantSource  string   `json:"wantSource"`
	EDNS        bool     `json:"edns"`
	UDPThenTCP  bool     `json:"udpThenTCP"`
	Clients     []string `json:"clients"`
	WantAnswers []WantRR `json:"wantAnswers"`
}

// WantRR is one expected answer RR.
type WantRR struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// Suite is testdata/interop/cases.json.
type Suite struct {
	Zone  string `json:"zone"`
	Notes string `json:"notes"`
	Cases []Case `json:"cases"`
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}

func loadSuite(t *testing.T) Suite {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "interop", "cases.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s Suite
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Cases) == 0 {
		t.Fatal("empty interop suite")
	}
	return s
}

func configPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "interop", "config.yaml")
}

func wantsClient(c Case, name string) bool {
	for _, cl := range c.Clients {
		if cl == name {
			return true
		}
	}
	return false
}

func rrType(s string) model.RRType {
	if s == "" {
		return model.TypeA
	}
	return model.RRType(s)
}
