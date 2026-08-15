package forwarder

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

var (
	// ErrNilSnapshot is returned when Exchange is called without a snapshot.
	ErrNilSnapshot = errors.New("forwarder: nil snapshot")
	// ErrUnknownPolicy is returned when the pre-selected policy ID is missing.
	ErrUnknownPolicy = errors.New("forwarder: unknown policy id")
	// ErrInvalidForwarding is returned by Compile for fail-closed invariants.
	ErrInvalidForwarding = errors.New("forwarder: invalid forwarding data")
)

// DefaultExchangeTimeout is the per-upstream exchange budget when
// FailoverSpec.Timeout is zero. Zero is not unlimited. 500ms is strictly
// less than dnsserver's 2s query timeout so OnTimeout can still try a
// second upstream inside the parent deadline.
const DefaultExchangeTimeout = 500 * time.Millisecond

// DefaultConnectTimeout is the Dial budget inside one attempt. It is
// capped by the remaining attempt (exchange) deadline.
const DefaultConnectTimeout = 250 * time.Millisecond

// ExchangeOpts are chaos/request-path hooks. Zero value is a normal exchange.
type ExchangeOpts struct {
	ForceUpstream model.UpstreamID
	Unavailable   map[model.UpstreamID]bool
}

// DialFunc opens a connected socket to a configured endpoint.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Runtime holds process-scoped picker, health, and dial state.
type Runtime struct {
	Clock  testutil.Clock
	Rand   testutil.Rand
	Health *Health
	Dial   DialFunc
	idSeq  atomic.Uint32

	pick *picker
}

// NewRuntime returns a Runtime. Nil clock/rand/dial use system defaults.
func NewRuntime(clk testutil.Clock, rng testutil.Rand, h *Health, dial DialFunc) *Runtime {
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	if rng == nil {
		rng = testutil.SystemRand{}
	}
	if h == nil {
		h = NewHealth(clk)
	}
	if dial == nil {
		dial = defaultDial
	}
	return &Runtime{
		Clock:  clk,
		Rand:   rng,
		Health: h,
		Dial:   dial,
		pick:   newPicker(rng, h),
	}
}

func defaultDial(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

// Exchange asks the pre-selected policy's pool for q. It never consults
// the host resolver. NXDOMAIN is a final answer and does not fail over.
func Exchange(ctx context.Context, snap *snapshot.Snapshot, q model.Query, policyID model.PolicyID) (model.Result, error) {
	return NewRuntime(nil, nil, nil, nil).Exchange(ctx, snap, q, policyID)
}

// Exchange implements suffix-selected forwarding for policyID.
func (rt *Runtime) Exchange(ctx context.Context, snap *snapshot.Snapshot, q model.Query, policyID model.PolicyID) (model.Result, error) {
	return rt.ExchangeOpts(ctx, snap, q, policyID, ExchangeOpts{})
}

// ExchangeOpts is Exchange plus chaos hooks.
func (rt *Runtime) ExchangeOpts(ctx context.Context, snap *snapshot.Snapshot, q model.Query, policyID model.PolicyID, opts ExchangeOpts) (model.Result, error) {
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	if snap == nil {
		return model.Result{}, ErrNilSnapshot
	}
	if rt == nil {
		rt = NewRuntime(nil, nil, nil, nil)
	}
	qname := canonicalSuffix(string(q.Name))
	if qname == "" {
		qname = "."
	}
	q.Name = qname
	if q.Class == "" {
		q.Class = model.ClassIN
	}

	pol, ok := snap.Forwarding.Lookup(policyID)
	if !ok {
		return servfail(snap, q, policyID, ""), fmt.Errorf("%w: %s", ErrUnknownPolicy, policyID)
	}
	pool, ok := snap.Forwarding.Pool(pol.PoolID)
	if !ok || pool == nil || len(pool.Upstreams) == 0 {
		return servfail(snap, q, policyID, ""), fmt.Errorf("%w: pool %s", ErrUnknownPolicy, pol.PoolID)
	}

	order := rt.pick.order(pool)
	if opts.ForceUpstream != "" {
		order = forceFirst(order, opts.ForceUpstream)
	}
	fo := pol.Failover
	attemptTO := fo.Timeout
	if attemptTO <= 0 {
		attemptTO = DefaultExchangeTimeout
	}

	var last model.Result
	for _, up := range order {
		if err := ctx.Err(); err != nil {
			return model.Result{}, err
		}
		if opts.Unavailable[up.ID] {
			last = servfail(snap, q, policyID, up.ID)
			if !fo.OnTransportError {
				break
			}
			continue
		}
		res, ferr, retryTCP := rt.attempt(ctx, q, up, attemptTO, fo.UDPTruncateRetryTCP)
		if retryTCP {
			tcp := up
			tcp.Transport = model.TransportTCP
			res, ferr, _ = rt.attempt(ctx, q, tcp, attemptTO, false)
		}
		if ferr != nil {
			rt.Health.RecordFailure(up.ID)
			// Parent total deadline expired: surface the error so dnsquery
			// can HintDrop instead of synthesizing SERVFAIL.
			if err := ctx.Err(); err != nil {
				return model.Result{}, err
			}
			last = servfail(snap, q, policyID, up.ID)
			if isTimeout(ferr) && !fo.OnTimeout {
				return last, nil
			}
			if !isTimeout(ferr) && !fo.OnTransportError {
				return last, nil
			}
			continue
		}
		rt.Health.RecordSuccess(up.ID)
		res.ForwardingID = policyID
		res.UpstreamID = up.ID
		res.Source = model.SourceUpstream
		res.AA = false
		res.AD = false
		res.RA = false
		// CD is whatever the upstream returned (pass-through).
		annotate(snap, q, &res, policyID, pool.ID, up.ID)
		last = res
		switch res.RCode {
		case model.RCodeServFail:
			if fo.OnSERVFAIL {
				continue
			}
		case model.RCodeRefused:
			if fo.OnREFUSED {
				continue
			}
		}
		return res, nil
	}
	if last.RCode == "" {
		last = servfail(snap, q, policyID, "")
	}
	return last, nil
}

func (rt *Runtime) attempt(ctx context.Context, q model.Query, up snapshot.CompiledUpstream, timeout time.Duration, allowTCPRetry bool) (model.Result, error, bool) {
	// Exchange deadline for this attempt; total deadline is the parent ctx.
	actx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	id := uint16(rt.idSeq.Add(1))
	if id == 0 {
		id = uint16(rt.idSeq.Add(1))
	}
	payload, err := dnswire.PackQuery(id, q, &dnswire.EDNS{UDPSize: 1232})
	if err != nil {
		return model.Result{}, err, false
	}

	network := "udp"
	if up.Transport == model.TransportTCP {
		network = "tcp"
	}
	connectTO := DefaultConnectTimeout
	if connectTO > timeout {
		connectTO = timeout
	}
	dctx, dcancel := context.WithTimeout(actx, connectTO)
	conn, err := rt.Dial(dctx, network, up.Endpoint)
	dcancel()
	if err != nil {
		return model.Result{}, err, false
	}
	defer func() { _ = conn.Close() }()

	if dl, ok := actx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	if network == "tcp" {
		var hdr [2]byte
		binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
		if _, err := conn.Write(hdr[:]); err != nil {
			return model.Result{}, err, false
		}
	}
	if _, err := conn.Write(payload); err != nil {
		return model.Result{}, err, false
	}

	var raw []byte
	if network == "tcp" {
		var hdr [2]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return model.Result{}, err, false
		}
		n := int(binary.BigEndian.Uint16(hdr[:]))
		if n == 0 {
			return model.Result{}, errors.New("forwarder: empty tcp response"), false
		}
		raw = make([]byte, n)
		if _, err := io.ReadFull(conn, raw); err != nil {
			return model.Result{}, err, false
		}
	} else {
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			return model.Result{}, err, false
		}
		raw = buf[:n]
	}

	upmsg, err := dnswire.UnpackUpstream(raw)
	if err != nil {
		return model.Result{}, err, false
	}
	if upmsg.ID != id {
		return model.Result{}, errors.New("forwarder: response id mismatch"), false
	}
	if upmsg.TC && allowTCPRetry && up.Transport == model.TransportUDP {
		return model.Result{}, nil, true
	}
	res := model.Result{
		RCode:      upmsg.RCode,
		Answers:    upmsg.Answers,
		Authority:  upmsg.Authority,
		Additional: upmsg.Additional,
		CD:         upmsg.CD,
		Source:     model.SourceUpstream,
		UpstreamID: up.ID,
	}
	return res, nil, false
}

func forceFirst(order []snapshot.CompiledUpstream, id model.UpstreamID) []snapshot.CompiledUpstream {
	var first *snapshot.CompiledUpstream
	rest := make([]snapshot.CompiledUpstream, 0, len(order))
	for i := range order {
		if order[i].ID == id && first == nil {
			u := order[i]
			first = &u
			continue
		}
		rest = append(rest, order[i])
	}
	if first == nil {
		return order
	}
	return append([]snapshot.CompiledUpstream{*first}, rest...)
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func servfail(snap *snapshot.Snapshot, q model.Query, policy model.PolicyID, up model.UpstreamID) model.Result {
	res := model.Result{
		RCode:        model.RCodeServFail,
		Source:       model.SourceUpstream,
		ForwardingID: policy,
		UpstreamID:   up,
		AA:           false,
		AD:           false,
		RA:           false,
		CD:           q.CD,
	}
	annotate(snap, q, &res, policy, "", up)
	return res
}

func annotate(snap *snapshot.Snapshot, q model.Query, res *model.Result, policy model.PolicyID, pool model.PoolID, up model.UpstreamID) {
	res.ForwardingID = policy
	res.UpstreamID = up
	rev := model.Revision("")
	if snap != nil {
		rev = snap.Revision
	}
	res.Explanation = &model.Explanation{
		Query:        q,
		Source:       res.Source,
		ForwardingID: policy,
		PoolID:       pool,
		UpstreamID:   up,
		Revision:     rev,
	}
}
