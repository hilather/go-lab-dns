package observability

import (
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// Level is a structured log severity.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Record is one structured event. QNAME and client addresses are omitted
// unless LogQNAME is enabled (debug, time-bounded by the caller).
type Record struct {
	Timestamp     time.Time `json:"timestamp"`
	Level         Level     `json:"level"`
	Event         string    `json:"event"`
	Component     string    `json:"component,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	TraceID       string    `json:"trace_id,omitempty"`
	StateRevision string    `json:"state_revision,omitempty"`
	Generation    uint64    `json:"generation,omitempty"`
	Transport     string    `json:"transport,omitempty"`
	Capability    string    `json:"capability,omitempty"`
	Result        string    `json:"result,omitempty"`
	ErrorCode     string    `json:"error_code,omitempty"`
	ZoneID        string    `json:"zone_id,omitempty"`
	PolicyID      string    `json:"policy_id,omitempty"`
	UpstreamID    string    `json:"upstream_id,omitempty"`
	DurationMS    float64   `json:"duration_ms,omitempty"`
	// QNAME is recorded only when Logger.LogQNAME is true.
	QNAME string `json:"qname,omitempty"`
	// Client is recorded only when Logger.LogQNAME is true (same debug gate).
	Client string `json:"client,omitempty"`
}

// Logger writes JSON events. The default path writes synchronously so
// records are not silently buffered. WithQueue enables a non-blocking
// buffer that drops (and counts) on overflow.
type Logger struct {
	mu       sync.Mutex
	out      io.Writer
	q        *Queue[Record]
	reg      *Registry
	now      func() time.Time
	dropped  atomic.Int64
	LogQNAME bool
	sync     bool
}

// NewLogger writes JSON lines to w on the calling goroutine. w may be nil (discard).
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		out:  w,
		now:  time.Now,
		sync: true,
	}
}

// WithSync writes on the calling goroutine (the NewLogger default).
func (l *Logger) WithSync() *Logger {
	if l != nil {
		l.sync = true
	}
	return l
}

// WithQueue switches Log to a bounded TrySend buffer. A full queue never
// blocks; overflow increments Logger.Dropped and labdns_telemetry_dropped_total
// when a Registry is attached via WithMetrics.
func (l *Logger) WithQueue(n int) *Logger {
	if l == nil {
		return nil
	}
	l.sync = false
	if l.q == nil {
		l.q = NewQueue[Record](n)
	}
	return l
}

// WithMetrics records queue overflow on the catalog drop counter.
func (l *Logger) WithMetrics(r *Registry) *Logger {
	if l != nil {
		l.reg = r
	}
	return l
}

// Queue is the optional async buffer.
func (l *Logger) Queue() *Queue[Record] {
	if l == nil {
		return nil
	}
	return l.q
}

// Log redacts sensitive fields and either writes or enqueues.
func (l *Logger) Log(rec Record) {
	if l == nil {
		return
	}
	if rec.Timestamp.IsZero() && l.now != nil {
		rec.Timestamp = l.now().UTC()
	}
	if rec.Level == "" {
		rec.Level = LevelInfo
	}
	rec = l.redact(rec)
	if l.sync || l.q == nil {
		l.write(rec)
		return
	}
	if !l.q.TrySend(rec) {
		l.dropped.Add(1)
		if l.reg != nil {
			l.reg.Inc(MetricTelemetryDropped, map[string]string{"reason": "log"}, 1)
		}
	}
}

// Dropped is the number of records discarded by a full WithQueue buffer.
func (l *Logger) Dropped() int64 {
	if l == nil {
		return 0
	}
	return l.dropped.Load()
}

// Drain writes queued records to the sink until q is empty. Non-blocking
// after the current buffer; intended for tests and shutdown.
func (l *Logger) Drain() {
	if l == nil || l.q == nil {
		return
	}
	for {
		select {
		case rec := <-l.q.Recv():
			l.write(rec)
		default:
			return
		}
	}
}

func (l *Logger) redact(rec Record) Record {
	if l != nil && l.LogQNAME {
		return rec
	}
	rec.QNAME = ""
	rec.Client = ""
	return rec
}

func (l *Logger) write(rec Record) {
	if l == nil || l.out == nil {
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(b)
	_, _ = l.out.Write([]byte("\n"))
}
