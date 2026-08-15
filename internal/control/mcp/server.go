package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ProtocolVersion is the only MCP revision first GA speaks (ADR 0006).
	ProtocolVersion = "2026-07-28"

	// SDKModule is the official Go SDK module path.
	SDKModule = "github.com/modelcontextprotocol/go-sdk"

	// SDKVersion is the pinned official SDK tag.
	SDKVersion = "v1.7.0"

	// DefaultPath is the Streamable HTTP mount on the management listener.
	DefaultPath = "/mcp"

	// DefaultMaxBodyBytes matches the REST management bound (1 MiB).
	DefaultMaxBodyBytes = 1 << 20

	// DefaultRequestTimeout is the per-request handler deadline.
	DefaultRequestTimeout = 30 * time.Second

	// DefaultMaxConcurrent is a coarse in-process admission cap until SEC-001.
	DefaultMaxConcurrent = 256

	headerProtocolVersion = "Mcp-Protocol-Version"
	headerRequestID       = "X-Request-ID"
	headerOrigin          = "Origin"
	headerAuthorization   = "Authorization"
)

// Authenticator validates a bearer token. PR-14 replaces the stub.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (auth.Actor, error)
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(ctx context.Context, token string) (auth.Actor, error)

// Authenticate calls f.
func (f AuthenticatorFunc) Authenticate(ctx context.Context, token string) (auth.Actor, error) {
	return f(ctx, token)
}

// Config constructs the MCP adapter.
type Config struct {
	// Service is required. Handlers call it and do not mutate snapshots.
	Service app.Service
	// Auth validates bearer tokens. Nil accepts any non-empty token (dev stub).
	Auth Authenticator
	// AllowedOrigins are extra Origins accepted besides loopback. Empty denies
	// every non-loopback Origin (DNS-rebinding default-deny).
	AllowedOrigins []string
	// MaxBodyBytes caps decoded request bodies. Non-positive uses DefaultMaxBodyBytes.
	MaxBodyBytes int64
	// RequestTimeout is the handler context deadline. Non-positive uses DefaultRequestTimeout.
	RequestTimeout time.Duration
	// MaxConcurrent admits at most this many overlapping requests. Non-positive uses DefaultMaxConcurrent.
	MaxConcurrent int
}

// Server is the official-SDK adapter. Third-party MCP types do not escape it.
type Server struct {
	cfg      Config
	svc      app.Service
	authn    Authenticator
	sdk      *sdk.Server
	http     *sdk.StreamableHTTPHandler
	maxBody  int64
	timeout  time.Duration
	inflight chan struct{}
	closed   atomic.Bool
}

type ctxKey int

const (
	ctxActor ctxKey = iota
	ctxRequestID
)

// New builds a Server. Tools, resources, and prompts come from the frozen registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("mcp: Service is required")
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

	info := buildinfo.Current()
	impl := &sdk.Implementation{
		Name:    "labdns",
		Title:   "LabDNS",
		Version: info.Version,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sdkSrv := sdk.NewServer(impl, &sdk.ServerOptions{
		Instructions: "LabDNS control plane. Use typed tools; do not assume connection state. Protocol " + ProtocolVersion + " only.",
		Logger:       logger,
		Capabilities: &sdk.ServerCapabilities{
			// Stateless first-GA: no list-changed push, no deprecated logging.
			Logging:   nil,
			Tools:     &sdk.ToolCapabilities{ListChanged: false},
			Prompts:   &sdk.PromptCapabilities{ListChanged: false},
			Resources: &sdk.ResourceCapabilities{ListChanged: false, Subscribe: false},
		},
		SchemaCache: sdk.NewSchemaCache(),
	})

	s := &Server{
		cfg:      cfg,
		svc:      cfg.Service,
		authn:    cfg.Auth,
		sdk:      sdkSrv,
		maxBody:  maxBody,
		timeout:  timeout,
		inflight: make(chan struct{}, n),
	}
	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	s.http = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server {
		return s.sdk
	}, &sdk.StreamableHTTPOptions{
		// 2026-07-28 Streamable HTTP is accepted only when Stateless is true.
		Stateless:                    true,
		Logger:                       logger,
		MaxRequestBodyBytes:          maxBody,
		PropagateRequestCancellation: true,
	})
	return s, nil
}

// Handler returns the Streamable HTTP adapter. Mount it at /mcp.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

// Close marks the adapter stopped. In-flight requests still run to completion.
func (s *Server) Close() {
	s.closed.Store(true)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)

	if r.Method == http.MethodOptions {
		writeRPC(w, http.StatusForbidden, domainerr.Forbidden("origin not allowed"))
		return
	}

	select {
	case s.inflight <- struct{}{}:
		defer func() { <-s.inflight }()
	default:
		writeRPC(w, http.StatusTooManyRequests, domainerr.RateLimited("too many concurrent management requests"))
		return
	}

	ctx := r.Context()
	var cancel context.CancelFunc
	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	ctx = context.WithValue(ctx, ctxRequestID, reqID)
	r = r.WithContext(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			writeRPC(w, http.StatusInternalServerError, domainerr.Internal("internal error"))
		}
	}()

	if err := validateOrigin(r, s.cfg.AllowedOrigins); err != nil {
		writeRPC(w, http.StatusForbidden, err)
		return
	}
	if err := validateProtocolVersion(r); err != nil {
		writeRPC(w, http.StatusBadRequest, err)
		return
	}

	actor, err := s.authenticate(r)
	if err != nil {
		status := http.StatusUnauthorized
		if de, ok := domainerr.As(err); ok && de.Code == domainerr.CodeForbidden {
			status = http.StatusForbidden
		}
		writeRPC(w, status, err)
		return
	}
	r = r.WithContext(withActor(r.Context(), actor))

	s.http.ServeHTTP(w, r)
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

func withActor(ctx context.Context, a auth.Actor) context.Context {
	return context.WithValue(ctx, ctxActor, a)
}

func actorFrom(ctx context.Context) auth.Actor {
	a, _ := ctx.Value(ctxActor).(auth.Actor)
	return a
}
