package observability

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxTraceID
	ctxSpanID
)

// Span is a sampled trace fragment. Attributes never include raw QNAME
// or client IP; names are hashed or omitted.
type Span struct {
	Name     string
	TraceID  string
	SpanID   string
	ParentID string
	Sampled  bool
	Attrs    map[string]string
}

// SpanSink receives finished spans. Implementations must not block.
type SpanSink interface {
	OnSpan(Span)
}

// Tracer is optional and sampled. SampleRate is in [0,1]; 0 disables.
type Tracer struct {
	SampleRate    float64
	HashSensitive bool
	sink          SpanSink
	q             *Queue[Span]
}

// NewTracer returns a disabled tracer (SampleRate 0) unless rate > 0.
func NewTracer(rate float64, sink SpanSink) *Tracer {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &Tracer{
		SampleRate: rate,
		sink:       sink,
		q:          NewQueue[Span](DefaultQueueSize),
	}
}

// Queue is the non-blocking span buffer.
func (t *Tracer) Queue() *Queue[Span] {
	if t == nil {
		return nil
	}
	return t.q
}

// Start returns a child span. When unsampled, the span is a no-op.
func (t *Tracer) Start(ctx context.Context, name string, attrs map[string]string) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	parent := TraceIDFrom(ctx)
	spanID := newID()
	sampled := t != nil && t.shouldSample(parent)
	sp := Span{
		Name:     name,
		TraceID:  parent,
		SpanID:   spanID,
		ParentID: SpanIDFrom(ctx),
		Sampled:  sampled,
		Attrs:    RedactAttrs(attrs, t != nil && t.HashSensitive),
	}
	if sp.TraceID == "" {
		sp.TraceID = spanID
	}
	ctx = WithTraceID(ctx, sp.TraceID)
	ctx = context.WithValue(ctx, ctxSpanID, sp.SpanID)
	return ctx, sp
}

// Finish delivers a sampled span to the sink/queue without blocking.
func (t *Tracer) Finish(sp Span) {
	if t == nil || !sp.Sampled {
		return
	}
	if t.q != nil {
		_ = t.q.TrySend(sp)
	}
	if t.sink != nil {
		t.sink.OnSpan(sp)
	}
}

func (t *Tracer) shouldSample(existingTrace string) bool {
	if t.SampleRate <= 0 {
		return false
	}
	if existingTrace != "" {
		// Stay consistent with the parent decision: a present trace id
		// from an upstream sampled request is kept.
		return true
	}
	if t.SampleRate >= 1 {
		return true
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return false
	}
	// Use the top 53 bits so the conversion fits exactly in a float64
	// mantissa (uint64(1)<<53).
	u := uint64(b[0])<<45 | uint64(b[1])<<37 | uint64(b[2])<<29 | uint64(b[3])<<21 | uint64(b[4])<<13 | uint64(b[5])<<5 | uint64(b[6])>>3
	return float64(u)/float64(uint64(1)<<53) < t.SampleRate
}

// RedactAttrs drops forbidden keys. When hash is true, QNAME-like values
// are replaced with a short SHA-256 prefix; otherwise they are omitted.
func RedactAttrs(in map[string]string, hash bool) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		lk := strings.ToLower(strings.TrimSpace(k))
		if ForbiddenLabel(lk) {
			if hash && (strings.Contains(lk, "qname") || lk == "name" || lk == "owner") {
				out[k+"_hash"] = HashName(v)
			}
			continue
		}
		out[k] = v
	}
	return out
}

// HashName is a short, non-reversible name token for debug traces.
func HashName(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

// WithRequestID stores a management/DNS request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxRequestID, id)
}

// RequestIDFrom returns the request id or "".
func RequestIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(ctxRequestID).(string)
	return s
}

// WithTraceID stores a trace id.
func WithTraceID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxTraceID, id)
}

// TraceIDFrom returns the trace id or "".
func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(ctxTraceID).(string)
	return s
}

// SpanIDFrom returns the current span id or "".
func SpanIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(ctxSpanID).(string)
	return s
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0"
	}
	return hex.EncodeToString(b[:])
}
