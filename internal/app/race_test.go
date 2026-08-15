package app

import (
	"context"
	"sync"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestConcurrentQueriesDuringApply(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				st, err := svc.GetState(ctx, actor())
				if err != nil {
					errCh <- err
					return
				}
				if st.Canonical == nil || len(st.Canonical.Spec.Zones) == 0 {
					errCh <- errTorn
					return
				}
				if st.RuntimeRevision == "" {
					errCh <- errTorn
					return
				}
				_, err = svc.Resolve(ctx, actor(), ResolveIn{Name: "ns1.lab.example.net.", Type: model.TypeA})
				if err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if svc.Store().Load().Revision == boot.Revision {
		t.Fatal("apply did not land")
	}
}

func TestConcurrentQueriesDuringReset(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				st, err := svc.GetState(ctx, actor())
				if err != nil {
					errCh <- err
					return
				}
				if st.Canonical == nil || len(st.Canonical.Spec.Zones) == 0 || st.RuntimeRevision == "" {
					errCh <- errTorn
					return
				}
				if _, err := svc.Resolve(ctx, actor(), ResolveIn{Name: "ns1.lab.example.net.", Type: model.TypeA}); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	if _, err := svc.Reset(ctx, actor(), ResetIn{Reason: "race"}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if svc.Store().Load().Revision != boot.Revision {
		t.Fatal("reset did not restore bootstrap")
	}
}

var errTorn = errString("torn snapshot observed")

type errString string

func (e errString) Error() string { return string(e) }
