package perf

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnsserver"
)

// Reference hardware assumed by the published first-GA baselines.
const (
	ReferenceCPU     = "2 vCPU (CI runner / developer laptop class)"
	ReferenceMemory  = "4 GiB"
	ReferenceOS      = "linux/amd64 or linux/arm64"
	DefaultSoak      = 2 * time.Second
	LongSoakHint     = "30m"
	BaselineRevision = "PERF-001 first GA"
)

// Safe default limits copied from the first-GA listeners and pack sample
// so capacity notes and tests stay aligned with production defaults.
const (
	SafeMaxConcurrentDelayed = 2000
	SafeMaxDelay             = 10 * time.Second
	SafeMaxInflight          = dnsserver.DefaultMaxInflight
	SafeMaxTCPConns          = dnsserver.DefaultMaxTCPConns
	SafeMaxTCPPerIP          = dnsserver.DefaultMaxTCPPerIP
	SafeCacheMaxEntries      = 10000
)

// EnvMetadata is pinned so a bench or soak log can be compared later.
type EnvMetadata struct {
	GOOS         string
	GOARCH       string
	GoVersion    string
	NumCPU       int
	GOMAXPROCS   int
	ReferenceCPU string
	ReferenceMem string
	Baseline     string
	RecordedAt   time.Time
}

// CaptureEnv records the current process environment. Wall time is the
// only non-reproducible field; benches print it for the operator log.
func CaptureEnv() EnvMetadata {
	return EnvMetadata{
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		ReferenceCPU: ReferenceCPU,
		ReferenceMem: ReferenceMemory,
		Baseline:     BaselineRevision,
		RecordedAt:   time.Now().UTC(),
	}
}

// String is a single-line, parse-friendly dump for bench logs.
func (m EnvMetadata) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "labdns-perf go=%s goos=%s goarch=%s cpus=%d gomaxprocs=%d ref_cpu=%q ref_mem=%q baseline=%s at=%s",
		m.GoVersion, m.GOOS, m.GOARCH, m.NumCPU, m.GOMAXPROCS, m.ReferenceCPU, m.ReferenceMem, m.Baseline, m.RecordedAt.Format(time.RFC3339))
	return b.String()
}
