package interop

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/perf"
)

func TestInteropFixturesGoWire(t *testing.T) {
	lab := startInterop(t)
	suite := loadSuite(t)
	for _, c := range suite.Cases {
		c := c
		if !wantsClient(c, "go") {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			runGoCase(t, lab, c)
		})
	}
}

func TestInteropFixturesOSResolver(t *testing.T) {
	lab := startInterop(t)
	suite := loadSuite(t)
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: time.Second}
			// Prefer TCP for the TC fixture so the Go resolver does not
			// have to rediscover truncation; other cases work on either.
			target := lab.UDPAddr().String()
			if strings.HasPrefix(network, "tcp") && lab.TCPAddr() != nil {
				target = lab.TCPAddr().String()
			}
			if strings.HasPrefix(network, "udp") {
				target = lab.UDPAddr().String()
			}
			return d.DialContext(ctx, network, target)
		},
	}
	for _, c := range suite.Cases {
		c := c
		if !wantsClient(c, "os") {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			runOSCase(t, r, c)
		})
	}
}

func startInterop(t *testing.T) *perf.Lab {
	t.Helper()
	st, err := config.LoadFile(configPath(t))
	if err != nil {
		t.Fatal(err)
	}
	return perf.NewLab(t, perf.Options{State: st, StartServer: true})
}

func runGoCase(t *testing.T, lab *perf.Lab, c Case) {
	t.Helper()
	q := model.Query{
		Name:      model.Name(c.Name),
		Type:      rrType(c.Type),
		Class:     model.ClassIN,
		Client:    netip.MustParseAddr("127.0.0.1"),
		Transport: model.TransportUDP,
		RD:        false,
	}
	var edns *dnswire.EDNS
	if c.EDNS {
		edns = &dnswire.EDNS{UDPSize: 1232}
	}
	raw := perf.PackQuery(t, 1, q, edns)
	udp := perf.MustExchangeUDP(t, lab.UDPAddr(), raw)

	if c.UDPThenTCP {
		if !perf.HasTC(udp) {
			t.Fatal("expected UDP TC")
		}
		tcpOut, err := perf.ExchangeTCP(t, lab.TCPAddr(), raw, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		msg := perf.Unpack(t, tcpOut)
		if string(msg.RCode) != c.WantRcode {
			t.Fatalf("tcp rcode=%s want %s", msg.RCode, c.WantRcode)
		}
		assertAnswers(t, msg, c)
		return
	}

	if c.WantTC && !perf.HasTC(udp) {
		t.Fatal("expected TC")
	}
	msg := perf.Unpack(t, udp)
	if string(msg.RCode) != c.WantRcode {
		t.Fatalf("rcode=%s want %s", msg.RCode, c.WantRcode)
	}
	if c.WantAA && !msg.AA {
		t.Fatal("expected AA")
	}
	if c.WantSOA && !hasType(msg.Authority, model.TypeSOA) {
		t.Fatalf("missing SOA in authority: %+v", msg.Authority)
	}
	if c.WantEDEText != "" && !perf.WireHasEDE(udp, uint16(c.WantEDECode), c.WantEDEText) {
		t.Fatalf("missing EDE code=%d text=%q", c.WantEDECode, c.WantEDEText)
	}
	if c.WantTTL > 0 && len(msg.Answers) > 0 {
		got := int(msg.Answers[0].TTL / time.Second)
		if got != c.WantTTL {
			t.Fatalf("ttl=%d want %d", got, c.WantTTL)
		}
	}
	if c.WantCNAME != "" && !hasCNAME(msg.Answers, c.WantCNAME) {
		t.Fatalf("missing CNAME %s in %+v", c.WantCNAME, msg.Answers)
	}
	assertAnswers(t, msg, c)
}

func runOSCase(t *testing.T, r *net.Resolver, c Case) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	host := strings.TrimSuffix(c.Name, ".")
	switch c.WantRcode {
	case "NXDOMAIN":
		_, err := r.LookupIP(ctx, "ip4", host)
		if err == nil {
			t.Fatal("expected NXDOMAIN error")
		}
		return
	case "SERVFAIL":
		_, err := r.LookupIP(ctx, "ip4", host)
		if err == nil {
			t.Fatal("expected SERVFAIL error")
		}
		return
	}
	switch c.Type {
	case "AAAA":
		ips, err := r.LookupIP(ctx, "ip6", host)
		if err == nil && len(ips) > 0 {
			t.Fatalf("NODATA AAAA returned %v", ips)
		}
		return
	case "TXT":
		txt, err := r.LookupTXT(ctx, host)
		if err != nil {
			t.Fatal(err)
		}
		if len(txt) == 0 {
			t.Fatal("empty TXT")
		}
		return
	default:
		ips, err := r.LookupIP(ctx, "ip4", host)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.WantAnswers) == 0 {
			return
		}
		want := lastA(c.WantAnswers)
		if want == "" {
			return
		}
		found := false
		for _, ip := range ips {
			if ip.String() == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("os resolver ips=%v want %s", ips, want)
		}
	}
}

func assertAnswers(t *testing.T, msg *dnswire.UpstreamMsg, c Case) {
	t.Helper()
	if c.WantAnswers == nil {
		return
	}
	if len(c.WantAnswers) == 0 {
		if len(msg.Answers) != 0 {
			t.Fatalf("want NODATA, answers=%+v", msg.Answers)
		}
		return
	}
	for _, want := range c.WantAnswers {
		if !hasAnswer(msg.Answers, model.RRType(want.Type), want.Data) {
			t.Fatalf("missing %s %q in %+v", want.Type, want.Data, msg.Answers)
		}
	}
}

func hasAnswer(rrs []model.RR, typ model.RRType, data string) bool {
	for _, rr := range rrs {
		if rr.Type != typ {
			continue
		}
		if data == "" || rr.Data == data || strings.Trim(rr.Data, `"`) == data {
			return true
		}
	}
	return false
}

func hasType(rrs []model.RR, typ model.RRType) bool {
	for _, rr := range rrs {
		if rr.Type == typ {
			return true
		}
	}
	return false
}

func hasCNAME(rrs []model.RR, target string) bool {
	want := strings.TrimSuffix(target, ".")
	for _, rr := range rrs {
		if rr.Type != model.TypeCNAME {
			continue
		}
		got := strings.TrimSuffix(rr.Data, ".")
		if strings.EqualFold(got, want) {
			return true
		}
	}
	return false
}

func lastA(rrs []WantRR) string {
	var last string
	for _, rr := range rrs {
		if rr.Type == "A" {
			last = rr.Data
		}
	}
	return last
}
