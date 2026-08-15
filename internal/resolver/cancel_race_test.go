package resolver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestResolveCanceledContext(t *testing.T) {
	snap := snapOf(t, []model.Zone{authZone(
		rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
	)}, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Resolve(ctx, snap, model.Query{Name: "ns1.lab.example.net.", Type: model.TypeA}, authZoneID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveConcurrentAndImmutable(t *testing.T) {
	zones := []model.Zone{authZone(
		rec("a", "ns1", model.TypeA, time.Second, "10.42.0.53"),
		rec("w", "*.tools", model.TypeA, time.Second, "10.42.0.20"),
		rec("c", "alias", model.TypeCNAME, time.Second, "ns1.lab.example.net."),
	)}
	snap := snapOf(t, zones, 0)
	// Mutate the source after compile; answers must stay frozen.
	zones[0].Records[0].Values[0] = "192.0.2.99"

	ctx := testutil.Context(t)
	var wg sync.WaitGroup
	const workers = 16
	const iters = 80
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				if ctx.Err() != nil {
					return
				}
				qname := "ns1.lab.example.net."
				typ := model.TypeA
				switch j % 4 {
				case 1:
					qname = "alpha.tools.lab.example.net."
				case 2:
					qname = "alias.lab.example.net."
				case 3:
					qname = "missing.lab.example.net."
				}
				res, err := Resolve(ctx, snap, model.Query{Name: model.Name(qname), Type: typ}, authZoneID)
				if err != nil {
					errCh <- err
					return
				}
				if qname == "ns1.lab.example.net." {
					if len(res.Answers) == 0 || res.Answers[0].Data != "10.42.0.53" {
						errCh <- errors.New("compiled data mutated or lost")
						return
					}
				}
				if res.AD || res.CD || res.RA {
					errCh <- errors.New("forged AD/CD/RA")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}
