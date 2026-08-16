package perf

import "testing"

func TestEnvMetadataPinned(t *testing.T) {
	m := CaptureEnv()
	if m.GOOS == "" || m.GOARCH == "" || m.GoVersion == "" {
		t.Fatalf("incomplete metadata: %+v", m)
	}
	if m.NumCPU < 1 || m.GOMAXPROCS < 1 {
		t.Fatalf("cpus=%d gomaxprocs=%d", m.NumCPU, m.GOMAXPROCS)
	}
	if m.Baseline != BaselineRevision {
		t.Fatalf("baseline=%s", m.Baseline)
	}
	t.Log(m.String())
}
