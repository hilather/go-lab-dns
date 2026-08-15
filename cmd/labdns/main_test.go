package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "labdns") {
		t.Fatalf("version output %q missing labdns", stdout.String())
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"labdns", "not-a-command"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestServeShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runContext(ctx, []string{"labdns", "serve"}, &stdout, &stderr)
	}()
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d, stderr=%q", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not shut down after cancel")
	}
	if !strings.Contains(stdout.String(), "shutting down") {
		t.Fatalf("stdout %q missing shutdown", stdout.String())
	}
}
