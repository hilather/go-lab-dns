package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func requireConfigFlag(args []string, name string, stderr io.Writer) (path string, rest []string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := fs.String("config", "", "path to bootstrap YAML or JSON")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	if *cfg == "" {
		_, _ = fmt.Fprintf(stderr, "labdns %s: --config is required\n", name)
		return "", nil, fmt.Errorf("missing --config")
	}
	return *cfg, fs.Args(), nil
}

func validateCmd(args []string, stdout, stderr io.Writer) int {
	path, _, err := requireConfigFlag(args, "validate", stderr)
	if err != nil {
		return 2
	}
	st, err := config.LoadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns validate: %v\n", err)
		return 1
	}
	rev, err := config.Revision(st)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns validate: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "ok revision=%s\n", rev)
	return 0
}

func canonicalizeCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("canonicalize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	format := fs.String("format", "yaml", "yaml or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labdns canonicalize: --config is required")
		return 2
	}
	st, err := config.LoadFile(*path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns canonicalize: %v\n", err)
		return 1
	}
	var body []byte
	switch strings.ToLower(*format) {
	case "json":
		body, err = config.CanonicalJSON(st)
	case "yaml", "yml", "":
		body, err = config.CanonicalYAML(st)
	default:
		_, _ = fmt.Fprintln(stderr, "labdns canonicalize: --format must be yaml or json")
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns canonicalize: %v\n", err)
		return 1
	}
	_, _ = stdout.Write(body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		_, _ = fmt.Fprintln(stdout)
	}
	return 0
}

func verifyCmd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	probesPath := fs.String("probes", "", "path to labdns.dev/probes/v1alpha1 YAML")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" || *probesPath == "" {
		_, _ = fmt.Fprintln(stderr, "labdns verify: --config and --probes are required")
		return 2
	}
	st, err := config.LoadFile(*path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns verify: load: %v\n", err)
		return 1
	}
	snap, err := compiler.Compile(ctx, st, compiler.CompileOpts{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns verify: compile: %v\n", err)
		return 1
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	svc := app.New(app.Options{Store: store, BootstrapPath: *path})
	doc, err := loadProbes(*probesPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns verify: probes: %v\n", err)
		return 1
	}
	failed := 0
	for _, p := range doc.Probes {
		if err := runProbe(ctx, svc, p); err != nil {
			_, _ = fmt.Fprintf(stderr, "FAIL %s: %v\n", p.ID, err)
			failed++
			continue
		}
		_, _ = fmt.Fprintf(stdout, "ok %s\n", p.ID)
	}
	if failed > 0 {
		_, _ = fmt.Fprintf(stderr, "labdns verify: %d/%d probes failed\n", failed, len(doc.Probes))
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "ok %d probes\n", len(doc.Probes))
	return 0
}

func queryCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "DNS name to query")
	typ := fs.String("type", "A", "RR type mnemonic")
	server := fs.String("server", "127.0.0.1:5353", "DNS server host:port")
	transport := fs.String("transport", "udp", "udp or tcp")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *name == "" {
		_, _ = fmt.Fprintln(stderr, "labdns query: --name is required")
		return 2
	}
	tr := model.Transport(strings.ToLower(*transport))
	switch tr {
	case model.TransportUDP, model.TransportTCP:
	default:
		_, _ = fmt.Fprintln(stderr, "labdns query: --transport must be udp or tcp")
		return 2
	}
	q := model.Query{
		Name:      model.Name(*name),
		Type:      model.RRType(strings.ToUpper(*typ)),
		Class:     model.ClassIN,
		RD:        true,
		Transport: tr,
	}
	raw, err := dnswire.PackQuery(1, q, nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns query: pack: %v\n", err)
		return 1
	}
	var resp []byte
	switch q.Transport {
	case model.TransportTCP:
		resp, err = exchangeTCP(*server, raw)
	default:
		resp, err = exchangeUDP(*server, raw)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns query: %v\n", err)
		return 1
	}
	msg, err := dnswire.UnpackUpstream(resp)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns query: unpack: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "rcode=%s aa=%v ra=%v\n", msg.RCode, msg.AA, msg.RA)
	for _, rr := range msg.Answers {
		_, _ = fmt.Fprintf(stdout, "%s %s %s %s\n", rr.Name, rr.TTL, rr.Type, rr.Data)
	}
	return 0
}

func healthcheckCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "http://127.0.0.1:8080/v1/health/ready", "management health URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *url == "" {
		_, _ = fmt.Fprintln(stderr, "labdns healthcheck: --url is required")
		return 2
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(*url)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = fmt.Fprintf(stderr, "labdns healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "ok")
	return 0
}

func exchangeUDP(addr string, payload []byte) ([]byte, error) {
	c, err := net.DialTimeout("udp", addr, time.Second)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write(payload); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func exchangeTCP(addr string, payload []byte) ([]byte, error) {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	if _, err := c.Write(append(hdr[:], payload...)); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n == 0 {
		return nil, fmt.Errorf("empty tcp response")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
