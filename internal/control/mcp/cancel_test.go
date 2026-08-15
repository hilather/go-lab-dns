package mcp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/buildinfo"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolCancellation(t *testing.T) {
	svc := mustBoot(t, copyNamedFixture(t, "empty-client-groups.yaml"))
	slow := &slowVersion{App: svc, started: make(chan struct{})}
	s, err := New(Config{Service: slow})
	if err != nil {
		t.Fatal(err)
	}
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	ctx, cancel := context.WithCancel(t.Context())
	errc := make(chan error, 1)
	go func() {
		_, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "dns_version_get", Arguments: map[string]any{}})
		errc <- err
	}()
	select {
	case <-slow.started:
	case <-time.After(2 * time.Second):
		t.Fatal("tool did not start")
	}
	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("canceled call returned nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled call did not return")
	}
	if slow.sawCancel.Load() == 0 {
		t.Fatal("service did not observe cancellation")
	}
}

type slowVersion struct {
	*app.App
	started   chan struct{}
	sawCancel atomic.Uint32
}

func (s *slowVersion) Version(ctx context.Context, actor auth.Actor) (*buildinfo.Info, error) {
	_ = actor
	select {
	case s.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		s.sawCancel.Store(1)
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		info := buildinfo.Current()
		return &info, nil
	}
}
