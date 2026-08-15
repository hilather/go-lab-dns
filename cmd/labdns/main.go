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
		_, _ = fmt.Fprintln(stderr, "usage: labdns <command>")
		_, _ = fmt.Fprintln(stderr, "commands: version, serve")
		_, _ = fmt.Fprintln(stderr, "serve: labdns serve --config PATH [--chaos-disable]")
		return 2
	}
	switch args[1] {
	case "version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Current().String())
		return 0
	case "serve":
		return serve(ctx, args[2:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		return 2
	}
}
