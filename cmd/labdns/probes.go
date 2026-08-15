package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/model"
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
	MatchedPolicy string   `yaml:"matchedPolicy"`
	MaximumDelay  string   `yaml:"maximumDelay"`
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
	return &doc, nil
}

func runProbe(ctx context.Context, svc *app.App, p probe) error {
	actor := auth.Actor{ID: "verify", Class: "startup"}
	qtype := model.RRType(strings.ToUpper(p.Query.Type))
	if qtype == "" {
		qtype = model.TypeA
	}
	if p.SimulateChaos {
		out, err := svc.SimulateChaos(ctx, actor, app.SimulateIn{
			Name:        model.Name(p.Query.Name),
			Type:        qtype,
			ClientGroup: model.ClientGroupID(p.Query.ClientGroup),
			Transport:   model.Transport(p.Query.Transport),
		})
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
	out, err := svc.Resolve(ctx, actor, app.ResolveIn{
		Name:        model.Name(p.Query.Name),
		Type:        qtype,
		ClientGroup: model.ClientGroupID(p.Query.ClientGroup),
		Transport:   model.Transport(p.Query.Transport),
	})
	if err != nil {
		return err
	}
	if out == nil {
		return fmt.Errorf("empty resolve result")
	}
	if p.Expect.RCode != "" && string(out.Result.RCode) != strings.ToUpper(p.Expect.RCode) {
		return fmt.Errorf("rcode=%s want=%s", out.Result.RCode, p.Expect.RCode)
	}
	if len(p.Expect.Answers) > 0 {
		got := map[string]bool{}
		for _, rr := range out.Result.Answers {
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
