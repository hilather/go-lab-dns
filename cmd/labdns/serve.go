package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/control/rest"
	"github.com/hilather/go-lab-dns/internal/dnsquery"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/observability"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

var _ dnsserver.Metrics = observability.DNSTransport{}

type serveFlags struct {
	Config           string
	ChaosDisable     bool
	DNSListen        string
	ManagementListen string
	ShutdownTimeout  time.Duration
	PIDFile          string
}

// serveRuntime is the process-local listeners and snapshot store.
type serveRuntime struct {
	dns     *dnsserver.Server
	mgmt    *rest.Server
	mgmtLn  net.Listener
	store   *snapshot.Store
	engine  *chaos.Engine
	app     *app.App
	snap    *snapshot.Snapshot
	stopSig func()
	pidPath string
}

func serve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, err := parseServeFlags(args, stderr)
	if err != nil {
		return 2
	}
	rt, err := serveFromConfig(ctx, flags)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labdns serve: %v\n", err)
		return 1
	}
	printListen(stdout, rt)
	<-ctx.Done()
	deadline := flags.ShutdownTimeout
	if deadline <= 0 {
		deadline = dnsserver.DefaultShutdownWait
	}
	shctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	_ = rt.Shutdown(shctx)
	_, _ = fmt.Fprintln(stdout, "labdns: shutting down")
	return 0
}

func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	disable := fs.Bool("chaos-disable", false, "inhibit chaos regardless of YAML (also LABDNS_CHAOS_DISABLE=1)")
	dnsListen := fs.String("dns-listen", "", "override DNS listen address (empty uses YAML, default :5353)")
	mgmtListen := fs.String("management-listen", "", "override management listen address; off/none/- leaves it unbound")
	shutdown := fs.Duration("shutdown-timeout", dnsserver.DefaultShutdownWait, "graceful shutdown deadline")
	pidFile := fs.String("pid-file", "", "write process id after listeners bind")
	if err := fs.Parse(args); err != nil {
		return serveFlags{}, err
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labdns serve: --config is required")
		return serveFlags{}, fmt.Errorf("missing --config")
	}
	return serveFlags{
		Config:           *path,
		ChaosDisable:     *disable || chaos.EnvChaosDisable(),
		DNSListen:        *dnsListen,
		ManagementListen: *mgmtListen,
		ShutdownTimeout:  *shutdown,
		PIDFile:          *pidFile,
	}, nil
}

// serveFromConfig loads, validates, and compiles path, installs the snapshot,
// then binds DNS and (unless unbound) management HTTP. It does not bind on
// any load/validate/compile error.
func serveFromConfig(ctx context.Context, flags serveFlags) (*serveRuntime, error) {
	path := flags.Config
	st, err := config.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	eng := chaos.NewEngine(nil, nil)
	snap, err := compiler.Compile(ctx, st, compiler.CompileOpts{EmergencyChaosOff: flags.ChaosDisable})
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	store := snapshot.NewStore()
	if flags.ChaosDisable {
		// YAML and ordinary apply cannot clear this process bit.
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
	h := dnsquery.New(store, eng, c, nil, nil)
	svc := app.New(app.Options{
		Store:         store,
		Cache:         c,
		Engine:        eng,
		BootstrapPath: path,
	})

	udpAddr, tcpAddr := dnsListenAddrs(snap)
	if flags.DNSListen != "" {
		udpAddr, tcpAddr = overrideDNSListen(flags.DNSListen, snap)
	}
	if udpAddr == "" && tcpAddr == "" {
		stopSig()
		return nil, fmt.Errorf("compile: no DNS protocol enabled")
	}
	srv, err := dnsserver.New(dnsserver.Config{
		UDPAddr: udpAddr,
		TCPAddr: tcpAddr,
		Handler: h,
		Metrics: observability.NewDNSTransport(reg),
	})
	if err != nil {
		stopSig()
		return nil, err
	}
	if err := srv.Start(); err != nil {
		stopSig()
		return nil, err
	}

	rt := &serveRuntime{
		dns:     srv,
		store:   store,
		engine:  eng,
		app:     svc,
		snap:    snap,
		stopSig: stopSig,
		pidPath: flags.PIDFile,
	}

	mgmtAddr, mgmtOff := managementListenAddr(snap, flags.ManagementListen)
	if !mgmtOff {
		mgmt, err := rest.New(rest.Config{Addr: mgmtAddr, Service: svc})
		if err != nil {
			_ = rt.Shutdown(context.Background())
			return nil, err
		}
		ln, err := net.Listen("tcp", mgmtAddr)
		if err != nil {
			_ = rt.Shutdown(context.Background())
			return nil, fmt.Errorf("management listen %s: %w", mgmtAddr, err)
		}
		rt.mgmt = mgmt
		rt.mgmtLn = ln
		go func() { _ = mgmt.Serve(ln) }()
	}

	if err := writePIDFile(flags.PIDFile); err != nil {
		_ = rt.Shutdown(context.Background())
		return nil, fmt.Errorf("pid-file: %w", err)
	}
	return rt, nil
}

func (r *serveRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if r.stopSig != nil {
		r.stopSig()
		r.stopSig = nil
	}
	// Cancel chaos sleeps before waiting on listeners so delayed queries
	// drain inside the shutdown deadline instead of the delay duration.
	if r.engine != nil {
		r.engine.CancelDelays()
	}
	var first error
	if r.mgmt != nil {
		if err := r.mgmt.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	} else if r.mgmtLn != nil {
		_ = r.mgmtLn.Close()
	}
	if r.dns != nil {
		if err := r.dns.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	if r.pidPath != "" {
		_ = os.Remove(r.pidPath)
	}
	return first
}

func (r *serveRuntime) UDPAddr() net.Addr {
	if r == nil || r.dns == nil {
		return nil
	}
	return r.dns.UDPAddr()
}

func (r *serveRuntime) TCPAddr() net.Addr {
	if r == nil || r.dns == nil {
		return nil
	}
	return r.dns.TCPAddr()
}

func (r *serveRuntime) MgmtAddr() string {
	if r == nil {
		return ""
	}
	if r.mgmtLn != nil {
		return r.mgmtLn.Addr().String()
	}
	if r.mgmt != nil {
		return r.mgmt.Addr()
	}
	return ""
}

func (r *serveRuntime) Store() *snapshot.Store {
	if r == nil {
		return nil
	}
	return r.store
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

func overrideDNSListen(addr string, snap *snapshot.Snapshot) (udpAddr, tcpAddr string) {
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

func managementListenAddr(snap *snapshot.Snapshot, override string) (addr string, off bool) {
	if managementOff(override) {
		return "", true
	}
	if override != "" {
		return override, false
	}
	addr = config.DefaultMgmtAddress
	if snap != nil && snap.Listeners.ManagementAddress != "" {
		addr = snap.Listeners.ManagementAddress
	}
	return addr, false
}

func managementOff(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "off", "none", "-", "unbound":
		return true
	default:
		return false
	}
}

func printListen(w io.Writer, rt *serveRuntime) {
	var rev model.Revision
	var udp, tcp, mgmt string
	if rt != nil {
		if rt.snap != nil {
			rev = rt.snap.Revision
		}
		if a := rt.UDPAddr(); a != nil {
			udp = a.String()
		}
		if a := rt.TCPAddr(); a != nil {
			tcp = a.String()
		}
		mgmt = rt.MgmtAddr()
	}
	if mgmt == "" {
		mgmt = "unbound"
	}
	_, _ = fmt.Fprintf(w, "labdns: listening udp %s tcp %s management %s revision %s\n", udp, tcp, mgmt, rev)
}

func writePIDFile(path string) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
