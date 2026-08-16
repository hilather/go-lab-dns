package perf

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestUpstreamOutageAndRecovery(t *testing.T) {
	up := StartFakeUpstream(t)
	up.SetAnswers(model.RR{Type: model.TypeA, Class: model.ClassIN, Data: "192.0.2.60"})
	lab := NewLab(t, Options{Upstream: up, DisableCache: true})

	ok := lab.Serve(t, QueryForward("svc.outside.example."))
	if ok.RCode != model.RCodeNoError || ok.Source != model.SourceUpstream {
		t.Fatalf("healthy: %+v", ok)
	}

	up.SetDown(true)
	down := lab.Serve(t, QueryForward("svc.outside.example."))
	if down.RCode != model.RCodeServFail {
		t.Fatalf("outage rcode=%s want SERVFAIL", down.RCode)
	}

	up.SetDown(false)
	up.SetAnswers(model.RR{Type: model.TypeA, Class: model.ClassIN, Data: "192.0.2.61"})
	// Give the health-unaware ordered pool a beat; Exchange uses the live
	// endpoint and should succeed on the next query.
	deadline := time.Now().Add(time.Second)
	var recovered model.Result
	for time.Now().Before(deadline) {
		recovered = lab.Serve(t, QueryForward("svc.outside.example."))
		if recovered.RCode == model.RCodeNoError {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if recovered.RCode != model.RCodeNoError {
		t.Fatalf("recovery rcode=%s", recovered.RCode)
	}
	if len(recovered.Answers) == 0 || recovered.Answers[0].Data != "192.0.2.61" {
		t.Fatalf("recovery answers %+v", recovered.Answers)
	}
}
