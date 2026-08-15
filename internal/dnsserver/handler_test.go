package dnsserver

import (
	"context"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestSetHintFailsAfterRelease(t *testing.T) {
	resp := NewResponse(model.Result{RCode: model.RCodeNoError})
	if err := resp.SetHint(HintDrop); err != nil {
		t.Fatal(err)
	}
	if resp.Hint() != HintDrop {
		t.Fatalf("hint=%s", resp.Hint())
	}
	resp.Release()
	if err := resp.SetHint(HintSend); err != ErrReleased {
		t.Fatalf("SetHint after release: %v", err)
	}
	if err := resp.SetHoldFor(time.Millisecond); err != ErrReleased {
		t.Fatalf("SetHoldFor after release: %v", err)
	}
	if err := resp.SetResult(model.Result{RCode: model.RCodeNXDomain}); err != ErrReleased {
		t.Fatalf("SetResult after release: %v", err)
	}
	if !resp.Released() {
		t.Fatal("Released()=false")
	}
	// Stored hint is unchanged after a failed SetHint.
	if resp.Hint() != HintDrop {
		t.Fatalf("hint mutated after release: %s", resp.Hint())
	}
}

func TestResolveHintReturnWinsUnlessSend(t *testing.T) {
	resp := NewResponse(model.Result{RCode: model.RCodeNoError})
	_ = resp.SetHint(HintDrop)
	if h := resolveHint(HintTCPReset, resp); h != HintTCPReset {
		t.Fatalf("returned hint should win: %s", h)
	}
	if !resp.Released() {
		t.Fatal("expected release")
	}
}

func TestResolveHintUsesResponseWhenReturnIsSend(t *testing.T) {
	resp := NewResponse(model.Result{RCode: model.RCodeNoError})
	_ = resp.SetHint(HintTruncate)
	if h := resolveHint(HintSend, resp); h != HintTruncate {
		t.Fatalf("got %s", h)
	}
}

func TestUnknownHintIsDrop(t *testing.T) {
	if h := resolveHint(TransportHint(99), nil); h != HintDrop {
		t.Fatalf("unknown hint=%s", h)
	}
}

func TestTCPHintOnUDPIsDrop(t *testing.T) {
	for _, h := range []TransportHint{HintTCPClose, HintTCPReset, HintHoldThenClose} {
		if got := applyTransportFallback(h, false); got != HintDrop {
			t.Fatalf("%s on udp: %s", h, got)
		}
	}
}

func TestTruncateOnTCPIsSend(t *testing.T) {
	if got := applyTransportFallback(HintTruncate, true); got != HintSend {
		t.Fatalf("truncate on tcp: %s", got)
	}
}

func TestHandlerFuncAndNilStub(t *testing.T) {
	var f HandlerFunc = func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
		return NewResponse(model.Result{RCode: model.RCodeRefused}), HintSend, nil
	}
	resp, hint, err := f.ServeDNS(context.Background(), &model.Query{Name: "x."})
	if err != nil || hint != HintSend || resp.Result().RCode != model.RCodeRefused {
		t.Fatalf("handlerfunc: %+v %s %v", resp, hint, err)
	}
	var stub *Stub
	resp, hint, err = stub.ServeDNS(context.Background(), &model.Query{Name: "x."})
	if err != nil || hint != HintSend || resp.Result().RCode != model.RCodeNXDomain {
		t.Fatalf("nil stub: %+v %s %v", resp.Result(), hint, err)
	}
}

func TestStubDelayCancels(t *testing.T) {
	s := &Stub{Delay: time.Second, RCode: model.RCodeNoError}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, hint, err := s.ServeDNS(ctx, &model.Query{Name: "x."})
	if err == nil || hint != HintDrop {
		t.Fatalf("want cancel drop, got hint=%s err=%v", hint, err)
	}
}

func TestNewRequiresHandlerAndAddr(t *testing.T) {
	if _, err := New(Config{UDPAddr: "127.0.0.1:0"}); err == nil {
		t.Fatal("expected handler required")
	}
	if _, err := New(Config{Handler: NXDOMAIN()}); err == nil {
		t.Fatal("expected addr required")
	}
}
