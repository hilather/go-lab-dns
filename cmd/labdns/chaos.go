package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func chaosCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: labdns chaos emergency-disable --pid-file PATH")
		return 2
	}
	switch args[0] {
	case "emergency-disable":
		return chaosEmergencyDisable(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		_, _ = fmt.Fprintln(stdout, "usage: labdns chaos emergency-disable --pid-file PATH")
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown chaos command: %s\n", args[0])
		_, _ = fmt.Fprintln(stderr, "usage: labdns chaos emergency-disable --pid-file PATH")
		return 2
	}
}

func chaosEmergencyDisable(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("chaos emergency-disable", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pidFile := fs.String("pid-file", "", "path written by labdns serve --pid-file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *pidFile == "" {
		_, _ = fmt.Fprintln(stderr, "labdns chaos emergency-disable: --pid-file is required")
		return 2
	}
	pid, err := readPIDFile(*pidFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns chaos emergency-disable: %v\n", err)
		return 1
	}
	// Local signal only: this path must work when management HTTP is down.
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns chaos emergency-disable: signal pid %d: %v\n", pid, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "sent SIGUSR1 to pid %d\n", pid)
	return 0
}

func readPIDFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	pid, err := strconv.Atoi(s)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid file %s", path)
	}
	return pid, nil
}
