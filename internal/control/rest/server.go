package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/audit"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/observability"
)

const (
	// DefaultAddr is the first-GA management listen address.
	DefaultAddr = ":8080"

	// DefaultMaxBodyBytes matches the config document bound (1 MiB).
	DefaultMaxBodyBytes = 1 << 20

	// DefaultRequestTimeout is the per-request handler deadline.
	DefaultRequestTimeout = 30 * time.Second

	// DefaultReadHeaderTimeout bounds slowloris-style header stalls.
	DefaultReadHeaderTimeout = 5 * time.Second

	// DefaultReadTimeout bounds the whole request read.
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout bounds the response write (docs can be large).
	DefaultWriteTimeout = 60 * time.Second

	// DefaultMaxConcurrent is the in-process overlapping-request cap.
	DefaultMaxConcurrent = 256

	headerRequestID   = "X-Request-ID"
	headerTraceID     = "X-Trace-ID"
	headerIdempotency = "Idempotency-Key"
	headerIfMatch     = "If-Match"
	headerExpected    = "X-LabDNS-Expected-Revision"
	headerRevision    = "X-LabDNS-Revision"
	headerAllow       = "Allow"

	requestURNPrefix = "urn:labdns:request:"

	cspValue = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
)

// Config constructs a management HTTP server.
type Config struct {
	// Addr is the listen address. Empty becomes DefaultAddr (:8080).
	Addr string
	// Service is required. Handlers call it and do not mutate snapshots.
	Service app.Service
	// Auth validates bearer tokens. Nil accepts any non-empty token as administrator.
	Auth Authenticator
	// Sessions is the in-process browser session table. Nil becomes an empty table.
	Sessions *auth.SessionTable
	// AllowedOrigins are extra Origins accepted besides loopback. Empty denies
	// every non-loopback Origin (CORS/DNS-rebinding default-deny). Used when Origins is nil.
	AllowedOrigins []string
	// Origins returns the per-request Origin allowlist from the active snapshot.
	// When non-nil it is preferred over AllowedOrigins.
	Origins func() []string
	// UIEnabled reports whether SPA assets should be served. Nil means true.
	UIEnabled func() bool
	// UI serves the embedded operator console on the pre-auth GET/HEAD branch.
	// Nil 404s those paths (PR-3). Injected by cmd/labdns; rest does not import internal/web.
	UI http.Handler
	// RatePerSec is the per-source management QPS. Zero uses auth.DefaultMgmtRatePerSec.
	// Negative disables the token bucket (concurrency cap still applies).
	RatePerSec float64
	// RateBurst is the per-source burst. Zero uses auth.DefaultMgmtBurst.
	RateBurst float64
	// Auditor receives denied-authorization events when Service does not
	// implement RecordAudit. *app.App writes the queryable ring; *audit.Fanout
	// implements Sink so DEP-001 can pass App.Audit() here.
	Auditor audit.Sink
	// Live overrides liveness. Nil is always live while the process serves.
	Live func() bool
	// Ready overrides readiness. Nil is app.Status.Ready (revision present
	// and required listeners bound; chaos does not flip it).
	Ready func() bool
	// MaxBodyBytes caps decoded request bodies. Non-positive uses DefaultMaxBodyBytes.
	MaxBodyBytes int64
	// RequestTimeout is the handler context deadline. Non-positive uses DefaultRequestTimeout.
	RequestTimeout time.Duration
	// ReadHeaderTimeout, ReadTimeout, WriteTimeout apply when ListenAndServe runs.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	// MaxConcurrent admits at most this many overlapping requests. Non-positive uses DefaultMaxConcurrent.
	MaxConcurrent int
	// Metrics records capability calls. Nil is a no-op.
	Metrics *observability.Registry
	// Logger is optional structured request logging. Nil is silent.
	Logger *observability.Logger
	// Tracer is optional sampled capability tracing. Nil disables.
	Tracer *observability.Tracer
	// Mounts are additional handlers served on the management listener ahead
	// of REST routing (http.ServeMux patterns, e.g. the MCP Streamable HTTP
	// adapter at /mcp). Mounted handlers own their auth, CORS, and limits;
	// paths must not overlap RESTPath routes.
	Mounts map[string]http.Handler
}

// Server is the stdlib net/http management listener.
type Server struct {
	cfg      Config
	svc      app.Service
	authn    Authenticator
	sessions *auth.SessionTable
	routes   []compiledRoute
	handler  http.Handler
	maxBody  int64
	timeout  time.Duration
	inflight chan struct{}
	metrics  *observability.Registry
	logger   *observability.Logger
	tracer   *observability.Tracer
	rate     *auth.Limiter

	mu     sync.Mutex
	http   *http.Server
	ln     net.Listener
	closed atomic.Bool
}

// New builds a Server. Routes come from the frozen capability registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("rest: Service is required")
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = DefaultMaxBodyBytes
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	n := cfg.MaxConcurrent
	if n <= 0 {
		n = DefaultMaxConcurrent
	}
	sessions := cfg.Sessions
	if sessions == nil {
		sessions = auth.NewSessionTable(auth.SessionTableConfig{})
	}
	s := &Server{
		cfg:      cfg,
		svc:      cfg.Service,
		authn:    cfg.Auth,
		sessions: sessions,
		routes:   compileRoutes(capabilities.All()),
		maxBody:  maxBody,
		timeout:  timeout,
		inflight: make(chan struct{}, n),
		metrics:  cfg.Metrics,
		logger:   cfg.Logger,
		tracer:   cfg.Tracer,
		rate:     auth.ManagementLimiter(cfg.RatePerSec, cfg.RateBurst, nil),
	}
	s.handler = http.HandlerFunc(s.serveHTTP)
	if len(cfg.Mounts) > 0 {
		mux := http.NewServeMux()
		for path, h := range cfg.Mounts {
			mux.Handle(path, h)
		}
		mux.Handle("/", http.HandlerFunc(s.serveHTTP))
		s.handler = mux
	}
	return s, nil
}

// Handler returns the management mux (REST routes plus any configured
// Mounts). Safe for httptest.NewServer / ServeHTTP.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// ListenAndServe binds Addr (default :8080) and serves until Shutdown.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.Addr
	if addr == "" {
		addr = DefaultAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve serves on ln until Shutdown. ln is closed by Shutdown or on return.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	if s.http != nil {
		s.mu.Unlock()
		_ = ln.Close()
		return errors.New("rest: server already started")
	}
	rh := s.cfg.ReadHeaderTimeout
	if rh <= 0 {
		rh = DefaultReadHeaderTimeout
	}
	rt := s.cfg.ReadTimeout
	if rt <= 0 {
		rt = DefaultReadTimeout
	}
	wt := s.cfg.WriteTimeout
	if wt <= 0 {
		wt = DefaultWriteTimeout
	}
	hs := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: rh,
		ReadTimeout:       rt,
		WriteTimeout:      wt,
		MaxHeaderBytes:    1 << 16,
	}
	s.http = hs
	s.ln = ln
	alreadyClosed := s.closed.Load()
	s.mu.Unlock()
	if alreadyClosed {
		_ = ln.Close()
		return nil
	}
	err := hs.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown closes the listener and waits for in-flight requests.
// If Serve has not stored http.Server yet, the listener is closed so a
// later Serve cannot accept.
func (s *Server) Shutdown(ctx context.Context) error {
	s.closed.Store(true)
	s.mu.Lock()
	hs := s.http
	ln := s.ln
	s.mu.Unlock()
	if hs != nil {
		return hs.Shutdown(ctx)
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// Addr returns the bound address after Serve, or the configured listen address.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	if s.cfg.Addr != "" {
		return s.cfg.Addr
	}
	return DefaultAddr
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)
	traceID := r.Header.Get(headerTraceID)
	if traceID != "" {
		w.Header().Set(headerTraceID, traceID)
	}
	ctx := r.Context()
	ctx = observability.WithRequestID(ctx, reqID)
	ctx = observability.WithTraceID(ctx, traceID)
	r = r.WithContext(ctx)
	instance := requestURNPrefix + reqID

	applySecurityHeaders(w.Header(), false)
	// Deny-all CORS: never emit allow-* headers.
	auth.ApplyCORS(w.Header())
	if err := auth.CheckOrigin(r.Header.Get("Origin"), s.origins()); err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	if r.Method == http.MethodOptions {
		s.writeProblem(w, r, instance, domainerr.Forbidden("CORS is disabled"))
		return
	}

	var cancel context.CancelFunc
	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(r.Context(), s.timeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	defer func() {
		if rec := recover(); rec != nil {
			s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		}
	}()

	if isSPACandidate(r.Method, r.URL.Path) {
		s.serveSPA(w, r, instance)
		return
	}

	rt, params, pathOK, methodOK := matchRoute(s.routes, r.Method, r.URL.Path)
	if !pathOK {
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
		return
	}
	if !methodOK {
		w.Header().Set(headerAllow, allowedMethods(s.routes, r.URL.Path))
		s.writeProblem(w, r, instance, domainerr.MethodNotAllowed("method not allowed"))
		return
	}

	select {
	case s.inflight <- struct{}{}:
		defer func() { <-s.inflight }()
	default:
		s.writeProblem(w, r, instance, domainerr.RateLimited("too many concurrent management requests"))
		return
	}

	if !isHealthCap(rt.cap) {
		if err := s.rate.AllowErr(auth.RateKey(r.RemoteAddr)); err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
	}

	actor, err := s.authenticate(r, rt.cap)
	if err != nil {
		if s.metrics != nil {
			result := "error"
			if de, ok := domainerr.As(err); ok {
				switch de.Code {
				case domainerr.CodeUnauthenticated:
					result = "unauthenticated"
				case domainerr.CodeForbidden:
					result = "forbidden"
				}
			}
			s.metrics.Inc(observability.MetricAuthFailures, map[string]string{"result": result}, 1)
		}
		s.auditDenied(r, actor, string(rt.cap.ID), err)
		s.writeProblem(w, r, instance, err)
		return
	}
	if err := auth.AuthorizeCapability(actor, rt.cap.RequiredScopes, string(rt.cap.ID)); err != nil {
		s.auditDenied(r, actor, string(rt.cap.ID), err)
		s.writeProblem(w, r, instance, err)
		return
	}
	if rt.cap.ID == capabilities.ChaosEmergency {
		enable := strings.HasSuffix(rt.path, ":emergency-enable")
		if err := auth.AuthorizeEmergency(actor, enable); err != nil {
			s.auditDenied(r, actor, string(rt.cap.ID), err)
			s.writeProblem(w, r, instance, err)
			return
		}
	}

	sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
	start := time.Now()
	if s.tracer != nil {
		var span observability.Span
		ctx, span = s.tracer.Start(r.Context(), string(rt.cap.ID), map[string]string{
			"capability": string(rt.cap.ID),
			"transport":  "rest",
		})
		r = r.WithContext(ctx)
		defer s.tracer.Finish(span)
	}
	s.dispatch(sw, r, instance, actor, rt, params)
	s.observe(rt, sw.code, time.Since(start), reqID)
}

func (s *Server) origins() []string {
	if s.cfg.Origins != nil {
		return s.cfg.Origins()
	}
	return s.cfg.AllowedOrigins
}

func (s *Server) uiEnabled() bool {
	if s.cfg.UIEnabled != nil {
		return s.cfg.UIEnabled()
	}
	return true
}

func isSPACandidate(method, path string) bool {
	if method != http.MethodGet && method != http.MethodHead {
		return false
	}
	if path == "/v1" || strings.HasPrefix(path, "/v1/") {
		return false
	}
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		return false
	}
	return true
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request, instance string) {
	applySecurityHeaders(w.Header(), true)
	if s.uiEnabled() && s.cfg.UI != nil {
		s.cfg.UI.ServeHTTP(w, r)
		return
	}
	s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
}

func applySecurityHeaders(h http.Header, html bool) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	if html {
		h.Set("Content-Security-Policy", cspValue)
	}
}

func isHealthCap(cap capabilities.Capability) bool {
	return cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady
}

func (s *Server) auditDenied(r *http.Request, actor auth.Actor, cap string, err error) {
	code := ""
	if de, ok := domainerr.As(err); ok {
		code = string(de.Code)
	}
	s.emitAudit(r.Context(), audit.Event{
		Time:       time.Now().UTC(),
		ActorID:    actor.ID,
		ActorClass: actor.Class,
		Transport:  "rest",
		Capability: cap,
		Result:     audit.ResultDenied,
		ErrorCode:  code,
	})
}

type auditRecorder interface {
	RecordAudit(context.Context, audit.Event) string
}

func (s *Server) emitAudit(ctx context.Context, ev audit.Event) {
	if rec, ok := s.svc.(auditRecorder); ok {
		rec.RecordAudit(ctx, ev)
		return
	}
	if s.cfg.Auditor != nil {
		_ = s.cfg.Auditor.Emit(ctx, ev)
	}
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(headerRequestID); id != "" {
		return id
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}

func (s *Server) isLive() bool {
	if s.cfg.Live != nil {
		return s.cfg.Live()
	}
	return !s.closed.Load()
}

func (s *Server) isReady(ctx context.Context) bool {
	if s.cfg.Ready != nil {
		return s.cfg.Ready()
	}
	st, err := s.svc.Status(ctx, auth.Actor{ID: "ready", Class: "startup"})
	return err == nil && st != nil && st.Ready
}

func (s *Server) observe(rt compiledRoute, code int, d time.Duration, reqID string) {
	result := "ok"
	if code >= 400 {
		result = "error"
	}
	if s.metrics != nil {
		s.metrics.Inc(observability.MetricCapabilityCalls, map[string]string{
			"capability": string(rt.cap.ID),
			"transport":  "rest",
			"result":     result,
		}, 1)
		s.metrics.Observe(observability.MetricCapabilityDuration, map[string]string{
			"capability": string(rt.cap.ID),
			"transport":  "rest",
		}, d.Seconds())
	}
	if s.logger != nil {
		s.logger.Log(observability.Record{
			Event:      observability.EventCapabilityInvoke,
			Component:  "rest",
			RequestID:  reqID,
			Capability: string(rt.cap.ID),
			Result:     result,
			DurationMS: float64(d.Milliseconds()),
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
