package dnsserver

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// Handler answers one admitted DNS query. Implementations must not retain
// the Query pointer after return and must not call transport actions on
// Response after ServeDNS returns.
type Handler interface {
	ServeDNS(ctx context.Context, req *model.Query) (*Response, TransportHint, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, req *model.Query) (*Response, TransportHint, error)

// ServeDNS calls f.
func (f HandlerFunc) ServeDNS(ctx context.Context, req *model.Query) (*Response, TransportHint, error) {
	return f(ctx, req)
}

// TransportHint is the chaos/transport action applied after ServeDNS.
type TransportHint int

const (
	// HintSend writes the encoded DNS response. Zero value.
	HintSend TransportHint = iota
	// HintDrop writes nothing.
	HintDrop
	// HintTruncate writes a syntactically valid TC response (UDP).
	HintTruncate
	// HintTCPClose closes a TCP connection with FIN and no DNS message.
	HintTCPClose
	// HintTCPReset aborts a TCP connection with RST and no DNS message.
	HintTCPReset
	// HintHoldThenClose waits a bounded time then closes TCP with no message.
	HintHoldThenClose
)

func (h TransportHint) String() string {
	switch h {
	case HintSend:
		return "send"
	case HintDrop:
		return "drop"
	case HintTruncate:
		return "truncate"
	case HintTCPClose:
		return "tcp-close"
	case HintTCPReset:
		return "tcp-reset"
	case HintHoldThenClose:
		return "hold-then-close"
	default:
		return "unknown"
	}
}

// Known reports whether h is a defined hint.
func (h TransportHint) Known() bool {
	return h >= HintSend && h <= HintHoldThenClose
}

// ErrReleased is returned when a transport action is used after the
// server has taken response ownership.
var ErrReleased = errors.New("dnsserver: response ownership released")

// Response is a model.Result plus transport-action state.
// miekg types never appear here.
type Response struct {
	mu       sync.Mutex
	result   model.Result
	hint     TransportHint
	holdFor  time.Duration
	released bool
}

// NewResponse owns result until ServeDNS returns.
func NewResponse(result model.Result) *Response {
	return &Response{result: result}
}

// Result returns a copy of the model result.
func (r *Response) Result() model.Result {
	if r == nil {
		return model.Result{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.result
}

// SetResult replaces the model result. Fails after Release.
func (r *Response) SetResult(res model.Result) error {
	if r == nil {
		return ErrReleased
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.released {
		return ErrReleased
	}
	r.result = res
	return nil
}

// Hint is the last SetHint value (HintSend if none).
func (r *Response) Hint() TransportHint {
	if r == nil {
		return HintSend
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hint
}

// SetHint records a transport action. Fails after Release.
func (r *Response) SetHint(h TransportHint) error {
	if r == nil {
		return ErrReleased
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.released {
		return ErrReleased
	}
	r.hint = h
	return nil
}

// HoldFor is the requested hold duration for HintHoldThenClose.
func (r *Response) HoldFor() time.Duration {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.holdFor
}

// SetHoldFor records a hold duration. Fails after Release.
func (r *Response) SetHoldFor(d time.Duration) error {
	if r == nil {
		return ErrReleased
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.released {
		return ErrReleased
	}
	r.holdFor = d
	return nil
}

// Released reports whether the server has taken ownership.
func (r *Response) Released() bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.released
}

// Release transfers ownership to the transport. Idempotent.
func (r *Response) Release() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.released = true
	r.mu.Unlock()
}

func resolveHint(returned TransportHint, resp *Response) TransportHint {
	if resp != nil {
		resp.Release()
		if returned == HintSend {
			if h := resp.Hint(); h != HintSend {
				returned = h
			}
		}
	}
	if !returned.Known() {
		// Unknown action: fail closed — do not send.
		return HintDrop
	}
	return returned
}

func applyTransportFallback(hint TransportHint, tcp bool) TransportHint {
	switch hint {
	case HintSend, HintDrop:
		return hint
	case HintTruncate:
		if tcp {
			// TC is a UDP signal. On TCP send the full response.
			return HintSend
		}
		return hint
	case HintTCPClose, HintTCPReset, HintHoldThenClose:
		if !tcp {
			// TCP-only action on UDP: drop, do not answer successfully.
			return HintDrop
		}
		return hint
	default:
		return HintDrop
	}
}
