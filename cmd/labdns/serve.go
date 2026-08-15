package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/dnsquery"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/observability"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

var _ dnsserver.Metrics = observability.DNSTransport{}

type serveFlags struct {
	Config       string
	ChaosDisable bool
}

func serve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, err := parseServeFlags(args, stderr)
	if err != nil {
		return 2
	}
	srv, snap, err := serveFromConfig(ctx, flags)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns serve: %v\n", err)
		return 1
	}
	printListen(stdout, srv, snap)
	<-ctx.Done()
	shctx, cancel := context.WithTimeout(context.Background(), dnsserver.DefaultShutdownWait)
	defer cancel()
	_ = srv.Shutdown(shctx)
	_, _ = fmt.Fprintln(stdout, "labdns: shutting down")
	return 0
}

func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	disable := fs.Bool("chaos-disable", false, "inhibit chaos regardless of YAML (also LABDNS_CHAOS_DISABLE=1)")
	if err := fs.Parse(args); err != nil {
		return serveFlags{}, err
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labdns serve: --config is required")
		return serveFlags{}, fmt.Errorf("missing --config")
	}
	return serveFlags{Config: *path, ChaosDisable: *disable || chaos.EnvChaosDisable()}, nil
}

// serveFromConfig loads, validates, and compiles path, installs the snapshot,
// then binds DNS. It does not bind on any load/validate/compile error.
func serveFromConfig(ctx context.Context, flags serveFlags) (*dnsserver.Server, *snapshot.Snapshot, error) {
	path := flags.Config
	st, err := config.LoadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", path, err)
	}
	eng := chaos.NewEngine(nil, nil)
	snap, err := compiler.Compile(ctx, st, compiler.CompileOpts{EmergencyChaosOff: flags.ChaosDisable})
	if err != nil {
		return nil, nil, fmt.Errorf("compile: %w", err)
	}
	store := snapshot.NewStore()
	if flags.ChaosDisable {
		store.SetEmergencyChaosOff(true)
	}
	store.InstallBootstrap(snap)

	sigCh, stopSig := chaos.NotifyUSR1()
	go func() {
		chaos.ServeSignals(ctx, sigCh, store, eng)
		stopSig()
	}()

	c := cache.New(cache.Policy{
		Enabled:            snap.CachePolicy.Enabled,
		MaxEntries:         snap.CachePolicy.MaxEntries,
		MinimumTTL:         snap.CachePolicy.MinimumTTL,
		MaximumTTL:         snap.CachePolicy.MaximumTTL,
		MaximumNegativeTTL: snap.CachePolicy.MaximumNegativeTTL,
		StaleServing:       snap.CachePolicy.StaleServing,
	}, nil)
	reg := observability.NewRegistry()
	h := dnsquery.NewOpts(dnsquery.Opts{Store: store, Engine: eng, Cache: c, Metrics: reg})

	udpAddr, tcpAddr := dnsListenAddrs(snap)
	if udpAddr == "" && tcpAddr == "" {
		return nil, nil, fmt.Errorf("compile: no DNS protocol enabled")
	}
	srv, err := dnsserver.New(dnsserver.Config{
		UDPAddr: udpAddr,
		TCPAddr: tcpAddr,
		Handler: h,
		Metrics: observability.NewDNSTransport(reg),
	})
	if err != nil {
		return nil, nil, err
	}
	if err := srv.Start(); err != nil {
		return nil, nil, err
	}
	return srv, snap, nil
}

func dnsListenAddrs(snap *snapshot.Snapshot) (udpAddr, tcpAddr string) {
	addr := config.DefaultDNSAddress
	if snap != nil && snap.Listeners.DNSAddress != "" {
		addr = snap.Listeners.DNSAddress
	}
	protos := []model.Transport{model.TransportUDP, model.TransportTCP}
	if snap != nil && len(snap.Listeners.DNSProtocols) > 0 {
		protos = snap.Listeners.DNSProtocols
	}
	for _, p := range protos {
		switch p {
		case model.TransportUDP:
			udpAddr = addr
		case model.TransportTCP:
			tcpAddr = addr
		}
	}
	return udpAddr, tcpAddr
}

func printListen(w io.Writer, srv *dnsserver.Server, snap *snapshot.Snapshot) {
	rev := model.Revision("")
	if snap != nil {
		rev = snap.Revision
	}
	var udp, tcp string
	if srv != nil {
		if a := srv.UDPAddr(); a != nil {
			udp = a.String()
		}
		if a := srv.TCPAddr(); a != nil {
			tcp = a.String()
		}
	}
	_, _ = fmt.Fprintf(w, "labdns: listening udp %s tcp %s revision %s\n", udp, tcp, rev)
}
