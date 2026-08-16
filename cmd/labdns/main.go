// Command labdns is the LabDNS process entrypoint.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/clihelp"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runContext(ctx, args, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	switch args[1] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Current().String())
		return 0
	case "serve":
		return serve(ctx, args[2:], stdout, stderr)
	case "validate":
		return validateCmd(args[2:], stdout, stderr)
	case "canonicalize":
		return canonicalizeCmd(args[2:], stdout, stderr)
	case "verify":
		return verifyCmd(ctx, args[2:], stdout, stderr)
	case "query":
		return queryCmd(args[2:], stdout, stderr)
	case "healthcheck":
		return healthcheckCmd(args[2:], stdout, stderr)
	case "chaos":
		return chaosCmd(args[2:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	_, _ = io.WriteString(w, clihelp.Text)
	_, _ = fmt.Fprintln(w, "usage: labdns <command>")
	_, _ = fmt.Fprintln(w, "commands:")
	_, _ = fmt.Fprintln(w, "  serve --config PATH [--chaos-disable] [--dns-listen ADDR] [--management-listen ADDR|off] [--shutdown-timeout DUR] [--pid-file PATH]")
	_, _ = fmt.Fprintln(w, "  validate --config PATH")
	_, _ = fmt.Fprintln(w, "  canonicalize --config PATH [--format yaml|json]")
	_, _ = fmt.Fprintln(w, "  verify --config PATH --probes PATH [--policies DIR] [--image REF|--image-env PATH] [--server HOST:PORT]")
	_, _ = fmt.Fprintln(w, "  query --name NAME [--type A] [--server HOST:PORT] [--transport udp|tcp]")
	_, _ = fmt.Fprintln(w, "  healthcheck --url URL")
	_, _ = fmt.Fprintln(w, "  chaos emergency-disable --pid-file PATH")
	_, _ = fmt.Fprintln(w, "  version")
	_, _ = fmt.Fprintln(w, "env: LABDNS_CHAOS_DISABLE=1 inhibits chaos at startup (YAML/API cannot relax it).")
	_, _ = fmt.Fprintln(w, "env: no environment variable raises chaos safety caps.")
}
