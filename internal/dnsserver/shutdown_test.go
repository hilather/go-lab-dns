package dnsserver

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestShutdownCancelsInFlight(t *testing.T) {
	started := make(chan struct{})
	s := startServer(t, Config{
		QueryTimeout: 5 * time.Second,
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			close(started)
			<-ctx.Done()
			return nil, HintDrop, ctx.Err()
		}),
	})
	go func() {
		_ = exchangeUDP(t, s.UDPAddr(), packA(t, "slow.lab.", 1, nil), 2*time.Second)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownIdempotentAndNoGoroutineLeak(t *testing.T) {
	runtime.GC()
	before := runtime.NumGoroutine()
	s := startServer(t, Config{Handler: NXDOMAIN()})
	_ = mustExchangeUDP(t, s.UDPAddr(), packA(t, "a.lab.", 1, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var after int
	for time.Now().Before(deadline) {
		after = runtime.NumGoroutine()
		if after <= before+3 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	t.Fatalf("goroutine leak: before=%d after=%d\n%s", before, after, buf[:n])
}

func TestConcurrentUDPAndShutdown(t *testing.T) {
	s := startServer(t, Config{Handler: StaticA("192.0.2.4")})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = exchangeUDP(t, s.UDPAddr(), packA(t, "c.lab.", uint16(i+1), nil), 500*time.Millisecond)
		}(i)
	}
	time.Sleep(10 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func TestQueryTimeoutCancelsHandler(t *testing.T) {
	s := startServer(t, Config{
		QueryTimeout: 40 * time.Millisecond,
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			<-ctx.Done()
			return nil, HintDrop, ctx.Err()
		}),
	})
	if out := exchangeUDP(t, s.UDPAddr(), packA(t, "to.lab.", 1, nil), 300*time.Millisecond); out != nil {
		t.Fatal("timed-out handler should drop")
	}
}

func TestHooksAnnotateContext(t *testing.T) {
	type snapKey struct{}
	type classKey struct{}
	var gotSnap, gotClass, gotPeer, gotUDP atomic.Bool
	s := startServer(t, Config{
		AcquireSnapshot: func(ctx context.Context) context.Context {
			return context.WithValue(ctx, snapKey{}, "snap")
		},
		ClassifySource: func(ctx context.Context, q *model.Query) context.Context {
			return context.WithValue(ctx, classKey{}, "lab")
		},
		Handler: HandlerFunc(func(ctx context.Context, q *model.Query) (*Response, TransportHint, error) {
			if ctx.Value(snapKey{}) == "snap" {
				gotSnap.Store(true)
			}
			if ctx.Value(classKey{}) == "lab" {
				gotClass.Store(true)
			}
			if _, ok := PeerAddr(ctx); ok {
				gotPeer.Store(true)
			}
			if tr, ok := TransportFromContext(ctx); ok && tr == model.TransportUDP {
				gotUDP.Store(true)
			}
			return NewResponse(model.Result{RCode: model.RCodeNXDomain}), HintSend, nil
		}),
	})
	_ = mustExchangeUDP(t, s.UDPAddr(), packA(t, "hook.lab.", 1, nil))
	if !gotSnap.Load() || !gotClass.Load() || !gotPeer.Load() || !gotUDP.Load() {
		t.Fatalf("hooks snap=%v class=%v peer=%v udp=%v", gotSnap.Load(), gotClass.Load(), gotPeer.Load(), gotUDP.Load())
	}
}
