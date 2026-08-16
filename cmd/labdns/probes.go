package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsquery"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"gopkg.in/yaml.v3"
)

const probesAPIVersion = "labdns.dev/probes/v1alpha1"

type probeDoc struct {
	APIVersion string  `yaml:"apiVersion"`
	Probes     []probe `yaml:"probes"`
}

type probe struct {
	ID            string      `yaml:"id"`
	SimulateChaos bool        `yaml:"simulateChaos"`
	Live          bool        `yaml:"live"`
	Query         probeQuery  `yaml:"query"`
	Expect        probeExpect `yaml:"expect"`
}

type probeQuery struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Transport   string `yaml:"transport"`
	ClientGroup string `yaml:"clientGroup"`
	Client      string `yaml:"client"`
}

type probeExpect struct {
	RCode         string   `yaml:"rcode"`
	Answers       []string `yaml:"answers"`
	AA            *bool    `yaml:"aa"`
	RA            *bool    `yaml:"ra"`
	NoUpstream    bool     `yaml:"noUpstream"`
	MatchedPolicy string   `yaml:"matchedPolicy"`
	MaximumDelay  string   `yaml:"maximumDelay"`
}

type probeRunner struct {
	svc   *app.App
	h     *dnsquery.Handler
	dials *atomic.Int64
}

func loadProbes(path string) (*probeDoc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc probeDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.APIVersion != "" && doc.APIVersion != probesAPIVersion {
		return nil, fmt.Errorf("unsupported probe apiVersion %q", doc.APIVersion)
	}
	if len(doc.Probes) == 0 {
		return nil, fmt.Errorf("no probes")
	}
	seen := map[string]bool{}
	for i, p := range doc.Probes {
		if strings.TrimSpace(p.ID) == "" {
			return nil, fmt.Errorf("probes[%d]: id is required", i)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("duplicate probe id %q", p.ID)
		}
		seen[p.ID] = true
	}
	return &doc, nil
}

func newProbeRunner(store *snapshot.Store, svc *app.App) *probeRunner {
	dials := &atomic.Int64{}
	fwd := forwarder.NewRuntime(nil, nil, nil, func(_ context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		return nil, fmt.Errorf("offline verify must not dial upstream %s %s", network, address)
	})
	h := dnsquery.NewOpts(dnsquery.Opts{
		Store:  store,
		Engine: chaos.NewEngine(nil, nil),
		Fwd:    fwd,
	})
	return &probeRunner{svc: svc, h: h, dials: dials}
}

func runProbe(ctx context.Context, r *probeRunner, p probe) error {
	if p.SimulateChaos {
		return runSimulateProbe(ctx, r.svc, p)
	}
	before := int64(0)
	if r.dials != nil {
		before = r.dials.Load()
	}
	q, err := probeQueryModel(p)
	if err != nil {
		return err
	}
	resp, _, err := r.h.ServeDNS(ctx, &q)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("empty data-plane result")
	}
	res := resp.Result()
	if err := checkResult(p, res); err != nil {
		return err
	}
	if p.Expect.NoUpstream && r.dials != nil && r.dials.Load() != before {
		return fmt.Errorf("noUpstream: forwarded to an upstream")
	}
	return nil
}

func runSimulateProbe(ctx context.Context, svc *app.App, p probe) error {
	actor := auth.Actor{ID: "verify", Class: "startup"}
	qtype := model.RRType(strings.ToUpper(p.Query.Type))
	if qtype == "" {
		qtype = model.TypeA
	}
	in := app.SimulateIn{
		Name:        model.Name(p.Query.Name),
		Type:        qtype,
		ClientGroup: model.ClientGroupID(p.Query.ClientGroup),
		Transport:   model.Transport(p.Query.Transport),
	}
	if addr, err := parseProbeClient(p.Query.Client); err != nil {
		return err
	} else {
		in.Client = addr
	}
	out, err := svc.SimulateChaos(ctx, actor, in)
	if err != nil {
		return err
	}
	if p.Expect.MatchedPolicy != "" {
		found := false
		for _, d := range out.Decisions {
			if string(d.PolicyID) == p.Expect.MatchedPolicy && d.Triggered {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("matchedPolicy %s not triggered", p.Expect.MatchedPolicy)
		}
	}
	if p.Expect.MaximumDelay != "" {
		// Simulation never sleeps; the field is accepted for document
		// compatibility and checked only as a parseable duration.
		if _, err := time.ParseDuration(p.Expect.MaximumDelay); err != nil {
			return fmt.Errorf("maximumDelay: %w", err)
		}
	}
	return nil
}

func runLiveProbe(p probe, server string) error {
	if server == "" {
		return fmt.Errorf("live probe requires --server")
	}
	q, err := probeQueryModel(p)
	if err != nil {
		return err
	}
	q.RD = true
	raw, err := dnswire.PackQuery(1, q, nil)
	if err != nil {
		return err
	}
	var resp []byte
	switch q.Transport {
	case model.TransportTCP:
		resp, err = exchangeTCP(server, raw)
	default:
		resp, err = exchangeUDP(server, raw)
	}
	if err != nil {
		return err
	}
	msg, err := dnswire.UnpackUpstream(resp)
	if err != nil {
		return err
	}
	return checkResult(p, model.Result{
		RCode:   msg.RCode,
		Answers: msg.Answers,
		AA:      msg.AA,
		RA:      msg.RA,
		AD:      msg.AD,
		CD:      msg.CD,
	})
}

func probeQueryModel(p probe) (model.Query, error) {
	qtype := model.RRType(strings.ToUpper(p.Query.Type))
	if qtype == "" {
		qtype = model.TypeA
	}
	tr := model.Transport(strings.ToLower(p.Query.Transport))
	if tr == "" {
		tr = model.TransportUDP
	}
	q := model.Query{
		Name:      model.Name(p.Query.Name),
		Type:      qtype,
		Class:     model.ClassIN,
		Transport: tr,
		RD:        true,
	}
	addr, err := parseProbeClient(p.Query.Client)
	if err != nil {
		return model.Query{}, err
	}
	q.Client = addr
	return q, nil
}

func parseProbeClient(s string) (netip.Addr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, nil
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("client: %w", err)
	}
	return addr, nil
}

func checkResult(p probe, res model.Result) error {
	if p.Expect.RCode != "" && string(res.RCode) != strings.ToUpper(p.Expect.RCode) {
		return fmt.Errorf("rcode=%s want=%s", res.RCode, p.Expect.RCode)
	}
	if p.Expect.AA != nil && res.AA != *p.Expect.AA {
		return fmt.Errorf("aa=%v want=%v", res.AA, *p.Expect.AA)
	}
	if p.Expect.RA != nil && res.RA != *p.Expect.RA {
		return fmt.Errorf("ra=%v want=%v", res.RA, *p.Expect.RA)
	}
	if len(p.Expect.Answers) > 0 {
		got := map[string]bool{}
		for _, rr := range res.Answers {
			got[rr.Data] = true
		}
		for _, want := range p.Expect.Answers {
			if !got[want] {
				return fmt.Errorf("missing answer %s", want)
			}
		}
	}
	return nil
}
