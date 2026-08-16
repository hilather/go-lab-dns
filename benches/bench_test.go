package benches

import (
	"context"
	"fmt"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/perf"
)

func TestEnv(t *testing.T) {
	t.Log(perf.CaptureEnv().String())
}

func BenchmarkExact(b *testing.B) {
	lab := perf.NewLab(b, perf.Options{})
	q := perf.QueryExact()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNoError || len(res.Answers) == 0 {
			b.Fatalf("rcode=%s answers=%d", res.RCode, len(res.Answers))
		}
	}
}

func BenchmarkWildcard(b *testing.B) {
	lab := perf.NewLab(b, perf.Options{})
	q := perf.QueryWildcard()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNoError || res.Source != model.SourceWildcard {
			b.Fatalf("rcode=%s source=%s", res.RCode, res.Source)
		}
	}
}

func BenchmarkNegative(b *testing.B) {
	lab := perf.NewLab(b, perf.Options{})
	q := perf.QueryNegative()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNXDomain {
			b.Fatalf("rcode=%s", res.RCode)
		}
	}
}

func BenchmarkCacheHit(b *testing.B) {
	lab := perf.NewLab(b, perf.Options{})
	q := perf.QueryExact()
	_ = lab.Serve(b, q)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.Source != model.SourceCache {
			b.Fatalf("source=%s", res.Source)
		}
	}
}

func BenchmarkCacheMiss(b *testing.B) {
	up := perf.StartFakeUpstream(b)
	up.SetAnswers(model.RR{Type: model.TypeA, Class: model.ClassIN, Data: "192.0.2.50"})
	lab := perf.NewLab(b, perf.Options{Upstream: up})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := perf.QueryForward(fmt.Sprintf("miss-%d.outside.example.", i))
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNoError {
			b.Fatalf("rcode=%s", res.RCode)
		}
	}
}

func BenchmarkUpstream(b *testing.B) {
	up := perf.StartFakeUpstream(b)
	up.SetAnswers(model.RR{Type: model.TypeA, Class: model.ClassIN, Data: "192.0.2.51"})
	lab := perf.NewLab(b, perf.Options{Upstream: up, DisableCache: true})
	q := perf.QueryForward("svc.outside.example.")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNoError || res.Source != model.SourceUpstream {
			b.Fatalf("rcode=%s source=%s", res.RCode, res.Source)
		}
	}
}

func BenchmarkChaosTriggered(b *testing.B) {
	st := perf.LabState("")
	// Keep the delay policy compiled but skip the sleep by using a zero
	// duration so this is a decision-path bench, not a wall-clock bench.
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = 0
	lab := perf.NewLab(b, perf.Options{State: st})
	q := perf.QueryDelay()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNoError {
			b.Fatalf("rcode=%s", res.RCode)
		}
	}
}

func BenchmarkChaosIdle(b *testing.B) {
	lab := perf.NewLab(b, perf.Options{})
	q := perf.QueryIdle()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := lab.Serve(b, q)
		if res.RCode != model.RCodeNoError {
			b.Fatalf("rcode=%s", res.RCode)
		}
	}
}

func TestHarnessPaths(t *testing.T) {
	up := perf.StartFakeUpstream(t)
	up.SetAnswers(model.RR{Type: model.TypeA, Class: model.ClassIN, Data: "192.0.2.9"})
	lab := perf.NewLab(t, perf.Options{Upstream: up})

	if res := lab.Serve(t, perf.QueryExact()); res.RCode != model.RCodeNoError || res.Source != model.SourceExact {
		t.Fatalf("exact %+v", res)
	}
	if res := lab.Serve(t, perf.QueryWildcard()); res.Source != model.SourceWildcard {
		t.Fatalf("wildcard source=%s", res.Source)
	}
	if res := lab.Serve(t, perf.QueryNegative()); res.RCode != model.RCodeNXDomain {
		t.Fatalf("nxdomain %s", res.RCode)
	}
	if res := lab.Serve(t, perf.QueryForward("one.outside.example.")); res.Source != model.SourceUpstream {
		t.Fatalf("upstream source=%s", res.Source)
	}
	if res := lab.Serve(t, perf.QueryForward("one.outside.example.")); res.Source != model.SourceCache {
		t.Fatalf("cache source=%s", res.Source)
	}
	if res := lab.Serve(t, perf.QueryIdle()); res.RCode != model.RCodeNoError {
		t.Fatalf("idle rcode=%s", res.RCode)
	}
	_, _, err := lab.ServeHint(context.Background(), perf.QueryDelay())
	if err != nil {
		t.Fatal(err)
	}
}
