package clihelp

import (
	"strings"
	"testing"
)

func TestTextHasRequiredCommandsAndEnv(t *testing.T) {
	for _, want := range []string{
		"usage: labdns",
		"serve --config PATH",
		"chaos emergency-disable --pid-file PATH",
		"LABDNS_CHAOS_DISABLE",
		"no environment variable raises chaos safety caps",
	} {
		if !strings.Contains(Text, want) {
			t.Errorf("CLI help missing %q", want)
		}
	}
	if !strings.HasSuffix(Text, "\n") {
		t.Fatal("CLI help must end with a newline")
	}
}
