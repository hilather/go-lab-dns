package perf

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/dnsquery"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Options builds a Lab.
type Options struct {
	State        *model.State
	Upstream     *FakeUpstream
	StartServer  bool
	DisableCache bool
	MaxInflight  int
	MaxTCPConns  int
	MaxTCPPerIP  int
	QueryTimeout time.Duration
	Clock        testutil.Clock
}

// Lab is one compiled snapshot, orchestrator, optional listeners, and
// optional fake upstream. Tests and benches share this so path coverage
// does not fork a second compile story.
type Lab struct {
	Store    *snapshot.Store
	Cache    *cache.Cache
	Engine   *chaos.Engine
	Handler  *dnsquery.Handler
	Server   *dnsserver.Server
	Upstream *FakeUpstream
	Clock    testutil.Clock
}

// NewLab compiles opts.State (or LabState) and wires dnsquery + optional
// UDP/TCP listeners on 127.0.0.1:0.
func NewLab(tb testing.TB, opts Options) *Lab {
	tb.Helper()
	clk := opts.Clock
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	up := opts.Upstream
	endpoint := ""
	if up != nil {
		endpoint = up.Addr()
	}
	st := opts.State
	if st == nil {
		st = LabState(endpoint)
	} else if endpoint != "" && len(st.Spec.Forwarding.Pools) > 0 && len(st.Spec.Forwarding.Pools[0].Upstreams) > 0 {
		st.Spec.Forwarding.Pools[0].Upstreams[0].Endpoint = endpoint
	}
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: clk})
	if err != nil {
		tb.Fatalf("compile: %v", err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	var c *cache.Cache
	if !opts.DisableCache {
		c = cache.New(cache.PolicyFromSpec(snap.Canonical.Spec.Cache), clk)
	}
	eng := chaos.NewEngine(clk, nil)
	h := dnsquery.NewOpts(dnsquery.Opts{
		Store:  store,
		Engine: eng,
		Cache:  c,
		Clock:  clk,
		Fwd:    forwarder.NewRuntime(clk, nil, nil, nil),
	})
	lab := &Lab{Store: store, Cache: c, Engine: eng, Handler: h, Upstream: up, Clock: clk}
	if opts.StartServer {
		cfg := dnsserver.Config{
			UDPAddr:      "127.0.0.1:0",
			TCPAddr:      "127.0.0.1:0",
			Handler:      h,
			MaxInflight:  opts.MaxInflight,
			MaxTCPConns:  opts.MaxTCPConns,
			MaxTCPPerIP:  opts.MaxTCPPerIP,
			QueryTimeout: opts.QueryTimeout,
			Clock:        clk,
		}
		srv, err := dnsserver.New(cfg)
		if err != nil {
			tb.Fatal(err)
		}
		if err := srv.Start(); err != nil {
			tb.Fatal(err)
		}
		testutil.Cleanup(tb, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		})
		lab.Server = srv
	}
	return lab
}

// Serve runs one in-process query. It fails the test on handler error or
// a non-send hint unless the caller uses ServeHint.
func (l *Lab) Serve(tb testing.TB, q model.Query) model.Result {
	tb.Helper()
	res, hint, err := l.Handler.ServeDNS(context.Background(), &q)
	if err != nil {
		tb.Fatal(err)
	}
	if hint != dnsserver.HintSend || res == nil {
		tb.Fatalf("hint=%s resp=%v", hint, res)
	}
	return res.Result()
}

// ServeHint is Serve without requiring HintSend (drop/truncate paths).
func (l *Lab) ServeHint(ctx context.Context, q model.Query) (*dnsserver.Response, dnsserver.TransportHint, error) {
	return l.Handler.ServeDNS(ctx, &q)
}

// Swap compiles st and atomically replaces the active snapshot. Queries
// in flight keep the snapshot they already loaded.
func (l *Lab) Swap(tb testing.TB, st *model.State) *snapshot.Snapshot {
	tb.Helper()
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{Clock: l.Clock})
	if err != nil {
		tb.Fatalf("compile swap: %v", err)
	}
	l.Store.Swap(snap)
	return snap
}

// UDPAddr is the bound UDP address.
func (l *Lab) UDPAddr() net.Addr {
	if l.Server == nil {
		return nil
	}
	return l.Server.UDPAddr()
}

// TCPAddr is the bound TCP address.
func (l *Lab) TCPAddr() net.Addr {
	if l.Server == nil {
		return nil
	}
	return l.Server.TCPAddr()
}

// EncodeQuery encodes a client query. Prefer this from worker goroutines
// (testing.TB.Fatal is not safe off the test goroutine).
func EncodeQuery(id uint16, q model.Query, edns *dnswire.EDNS) ([]byte, error) {
	return dnswire.PackQuery(id, q, edns)
}

// PackQuery encodes a client query, optionally with EDNS.
func PackQuery(tb testing.TB, id uint16, q model.Query, edns *dnswire.EDNS) []byte {
	tb.Helper()
	raw, err := EncodeQuery(id, q, edns)
	if err != nil {
		tb.Fatal(err)
	}
	return raw
}

// DialUDP writes payload and waits up to d for a reply. Nil body means
// drop or timeout; a non-nil error is a local dial/write failure.
func DialUDP(addr net.Addr, payload []byte, d time.Duration) ([]byte, error) {
	c, err := net.Dial("udp", addr.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(d))
	if _, err := c.Write(payload); err != nil {
		return nil, err
	}
	buf := make([]byte, 8192)
	n, err := c.Read(buf)
	if err != nil {
		return nil, nil
	}
	return buf[:n], nil
}

// ExchangeUDP writes payload and waits up to d for a reply. Nil means drop
// or timeout (admission / chaos silence).
func ExchangeUDP(tb testing.TB, addr net.Addr, payload []byte, d time.Duration) []byte {
	tb.Helper()
	out, err := DialUDP(addr, payload, d)
	if err != nil {
		tb.Fatal(err)
	}
	return out
}

// MustExchangeUDP fails if no reply arrives within a second.
func MustExchangeUDP(tb testing.TB, addr net.Addr, payload []byte) []byte {
	tb.Helper()
	out := ExchangeUDP(tb, addr, payload, time.Second)
	if out == nil {
		tb.Fatal("udp: no response")
	}
	return out
}

// ExchangeTCP writes a length-prefixed query and reads one response.
func ExchangeTCP(tb testing.TB, addr net.Addr, payload []byte, d time.Duration) ([]byte, error) {
	tb.Helper()
	c, err := net.Dial("tcp", addr.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(d))
	var hdr [2]byte
	hdr[0] = byte(len(payload) >> 8)
	hdr[1] = byte(len(payload))
	if _, err := c.Write(hdr[:]); err != nil {
		return nil, err
	}
	if _, err := c.Write(payload); err != nil {
		return nil, err
	}
	if _, err := readFull(c, hdr[:]); err != nil {
		return nil, err
	}
	n := int(hdr[0])<<8 | int(hdr[1])
	body := make([]byte, n)
	if n > 0 {
		if _, err := readFull(c, body); err != nil {
			return nil, err
		}
	}
	return body, nil
}

func readFull(c net.Conn, buf []byte) (int, error) {
	off := 0
	for off < len(buf) {
		n, err := c.Read(buf[off:])
		off += n
		if err != nil {
			return off, err
		}
	}
	return off, nil
}

// Unpack is a test helper around dnswire.UnpackUpstream.
func Unpack(tb testing.TB, raw []byte) *dnswire.UpstreamMsg {
	tb.Helper()
	msg, err := dnswire.UnpackUpstream(raw)
	if err != nil {
		tb.Fatal(err)
	}
	return msg
}

// HasTC reports the wire TC bit.
func HasTC(msg []byte) bool {
	return len(msg) >= 3 && msg[2]&0x02 != 0
}

// RcodeOf returns the 4-bit header RCODE.
func RcodeOf(msg []byte) byte {
	if len(msg) < 4 {
		return 0xff
	}
	return msg[3] & 0x0F
}

// WireHasEDE is a byte-level RFC 8914 option-15 search so tests do not
// import miekg outside dnswire.
func WireHasEDE(raw []byte, code uint16, text string) bool {
	opt := make([]byte, 6+len(text))
	opt[0], opt[1] = 0, 15
	opt[2] = byte((2 + len(text)) >> 8)
	opt[3] = byte(2 + len(text))
	opt[4] = byte(code >> 8)
	opt[5] = byte(code)
	copy(opt[6:], text)
	if len(opt) == 0 || len(raw) < len(opt) {
		return false
	}
	for i := 0; i+len(opt) <= len(raw); i++ {
		ok := true
		for j := range opt {
			if raw[i+j] != opt[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
