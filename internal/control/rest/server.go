package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
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

	// DefaultMaxConcurrent is a coarse in-process admission cap until SEC-001.
	DefaultMaxConcurrent = 256

	headerRequestID   = "X-Request-ID"
	headerTraceID     = "X-Trace-ID"
	headerIdempotency = "Idempotency-Key"
	headerIfMatch     = "If-Match"
	headerExpected    = "X-LabDNS-Expected-Revision"
	headerRevision    = "X-LabDNS-Revision"
	headerAllow       = "Allow"

	requestURNPrefix = "urn:labdns:request:"
)

// Config constructs a management HTTP server.
type Config struct {
	// Addr is the listen address. Empty becomes DefaultAddr (:8080).
	Addr string
	// Service is required. Handlers call it and do not mutate snapshots.
	Service app.Service
	// Auth validates bearer tokens. Nil accepts any non-empty token (dev stub).
	Auth Authenticator
	// Live overrides liveness. Nil is always live while the process serves.
	Live func() bool
	// Ready overrides readiness. Nil is "Status reports a runtime revision".
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
}

// Server is the stdlib net/http management listener.
type Server struct {
	cfg      Config
	svc      app.Service
	authn    Authenticator
	routes   []compiledRoute
	maxBody  int64
	timeout  time.Duration
	inflight chan struct{}

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
	s := &Server{
		cfg:      cfg,
		svc:      cfg.Service,
		authn:    cfg.Auth,
		routes:   compileRoutes(capabilities.All()),
		maxBody:  maxBody,
		timeout:  timeout,
		inflight: make(chan struct{}, n),
	}
	return s, nil
}

// Handler returns the management mux. Safe for httptest.NewServer / ServeHTTP.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
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
	s.mu.Unlock()
	err := hs.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown closes the listener and waits for in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.closed.Store(true)
	s.mu.Lock()
	hs := s.http
	s.mu.Unlock()
	if hs == nil {
		return nil
	}
	return hs.Shutdown(ctx)
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
	if tr := r.Header.Get(headerTraceID); tr != "" {
		w.Header().Set(headerTraceID, tr)
	}
	instance := requestURNPrefix + reqID

	// Never advertise a permissive CORS policy.
	if r.Method == http.MethodOptions {
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
		return
	}

	select {
	case s.inflight <- struct{}{}:
		defer func() { <-s.inflight }()
	default:
		s.writeProblem(w, r, instance, domainerr.RateLimited("too many concurrent management requests"))
		return
	}

	ctx := r.Context()
	var cancel context.CancelFunc
	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	r = r.WithContext(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		}
	}()

	rt, params, pathOK, methodOK := matchRoute(s.routes, r.Method, r.URL.Path)
	if !pathOK {
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
		return
	}
	if !methodOK {
		w.Header().Set(headerAllow, allowedMethods(s.routes, r.URL.Path))
		s.writeStatusProblem(w, r, instance, domainerr.ValidationFailed("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	actor, err := s.authenticate(r, rt.cap)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}

	s.dispatch(w, r, instance, actor, rt, params)
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
	return err == nil && st != nil && st.Revisions.RuntimeRevision != ""
}
