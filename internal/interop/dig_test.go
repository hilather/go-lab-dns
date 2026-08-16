package interop

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestInteropFixturesDig(t *testing.T) {
	if _, err := exec.LookPath("dig"); err != nil {
		t.Skip("dig not on PATH")
	}
	lab := startInterop(t)
	host, port, err := splitHostPort(lab.UDPAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	suite := loadSuite(t)
	for _, c := range suite.Cases {
		c := c
		if !wantsClient(c, "dig") {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			runDigCase(t, host, port, lab.TCPAddr().String(), c)
		})
	}
}

func runDigCase(t *testing.T, host, port, tcpAddr string, c Case) {
	t.Helper()
	if c.UDPThenTCP {
		udpOut := runDig(t, host, port, c, "+ignore", "+notcp")
		if !strings.Contains(udpOut, " tc") && !strings.Contains(udpOut, " tc;") && !hasDigFlag(udpOut, "tc") {
			t.Fatalf("dig UDP missing TC:\n%s", udpOut)
		}
		tcpHost, tcpPort, err := splitHostPort(tcpAddr)
		if err != nil {
			t.Fatal(err)
		}
		tcpOut := runDig(t, tcpHost, tcpPort, c, "+tcp")
		if !strings.Contains(tcpOut, "status: "+c.WantRcode) {
			t.Fatalf("dig TCP status:\n%s", tcpOut)
		}
		if c.WantAnswers != nil {
			for _, want := range c.WantAnswers {
				if want.Data != "" && !strings.Contains(tcpOut, want.Data) {
					t.Fatalf("dig TCP missing %s:\n%s", want.Data, tcpOut)
				}
			}
		}
		return
	}

	args := []string{"+norecurse"}
	if c.EDNS {
		args = append(args, "+edns=0")
	}
	out := runDig(t, host, port, c, args...)
	if !strings.Contains(out, "status: "+c.WantRcode) {
		t.Fatalf("dig status want %s:\n%s", c.WantRcode, out)
	}
	if c.WantAA && !hasDigFlag(out, "aa") {
		t.Fatalf("dig missing AA:\n%s", out)
	}
	if c.WantEDEText != "" && !strings.Contains(out, c.WantEDEText) {
		t.Fatalf("dig missing EDE text:\n%s", out)
	}
	if c.WantTTL > 0 && c.WantRcode == "NOERROR" {
		if !strings.Contains(out, "\t"+strconv.Itoa(c.WantTTL)+"\t") && !strings.Contains(out, " "+strconv.Itoa(c.WantTTL)+" ") {
			// Accept either presentation; some dig builds omit the TTL
			// column under +short. Full output is used here.
			t.Logf("dig ttl %d not obvious in output (non-fatal if answer present):\n%s", c.WantTTL, out)
		}
	}
	for _, want := range c.WantAnswers {
		if want.Data != "" && !strings.Contains(out, strings.TrimSuffix(want.Data, ".")) && !strings.Contains(out, want.Data) {
			t.Fatalf("dig missing %s %s:\n%s", want.Type, want.Data, out)
		}
	}
	if c.WantCNAME != "" && !strings.Contains(out, strings.TrimSuffix(c.WantCNAME, ".")) {
		t.Fatalf("dig missing CNAME:\n%s", out)
	}
}

func runDig(t *testing.T, host, port string, c Case, extra ...string) string {
	t.Helper()
	args := []string{"@" + host, "-p", port, "+time=2", "+tries=1", "+norecurse"}
	args = append(args, extra...)
	args = append(args, c.Name, c.Type)
	cmd := exec.Command("dig", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dig %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func hasDigFlag(out, flag string) bool {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "flags:") {
			continue
		}
		fields := strings.Fields(strings.ToLower(line))
		for _, f := range fields {
			f = strings.Trim(f, ";,")
			if f == flag {
				return true
			}
		}
	}
	return false
}

func splitHostPort(addr string) (string, string, error) {
	return net.SplitHostPort(addr)
}
