package dnsserver

import (
	"context"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Default listener and admission limits (first GA).
const (
	DefaultMaxUDPSize      = 4096
	DefaultMaxTCPSize      = 65535
	DefaultMaxQuestions    = 1
	DefaultMaxEDNSUDPSize  = 4096
	DefaultAdvertisedEDNS  = 1232
	DefaultTCPReadTimeout  = 2 * time.Second
	DefaultTCPWriteTimeout = 2 * time.Second
	DefaultTCPIdleTimeout  = 10 * time.Second
	DefaultTCPMaxAge       = 30 * time.Second
	DefaultQueryTimeout    = 2 * time.Second
	DefaultMaxTCPConns     = 256
	DefaultMaxTCPPerIP     = 16
	DefaultMaxInflight     = 1024
	DefaultMaxHold         = time.Second
	DefaultShutdownWait    = 5 * time.Second
)

// Config is the listener and admission configuration.
type Config struct {
	// UDPAddr and TCPAddr are host:port values. Empty disables that
	// protocol. ":0" / "127.0.0.1:0" selects an ephemeral port.
	UDPAddr string
	TCPAddr string

	Handler Handler
	Metrics Metrics
	Clock   testutil.Clock

	MaxUDPSize        int
	MaxTCPSize        int
	MaxQuestions      int
	MaxEDNSUDPSize    uint16
	AdvertisedEDNSUDP uint16
	TCPReadTimeout    time.Duration
	TCPWriteTimeout   time.Duration
	TCPIdleTimeout    time.Duration
	TCPMaxAge         time.Duration
	QueryTimeout      time.Duration
	MaxTCPConns       int
	MaxTCPPerIP       int
	MaxInflight       int
	MaxHold           time.Duration
	ShutdownTimeout   time.Duration

	// AcquireSnapshot may attach a snapshot handle to ctx after admission.
	// Transport does not load or inspect snapshots.
	AcquireSnapshot func(ctx context.Context) context.Context
	// ClassifySource may attach client-group classification to ctx.
	// Transport does not implement access policy.
	ClassifySource func(ctx context.Context, q *model.Query) context.Context
}

func (c Config) withDefaults() Config {
	if c.Metrics == nil {
		c.Metrics = NopMetrics{}
	}
	if c.MaxUDPSize <= 0 {
		c.MaxUDPSize = DefaultMaxUDPSize
	}
	if c.MaxTCPSize <= 0 {
		c.MaxTCPSize = DefaultMaxTCPSize
	}
	if c.MaxQuestions <= 0 {
		c.MaxQuestions = DefaultMaxQuestions
	}
	if c.MaxEDNSUDPSize == 0 {
		c.MaxEDNSUDPSize = DefaultMaxEDNSUDPSize
	}
	if c.AdvertisedEDNSUDP == 0 {
		c.AdvertisedEDNSUDP = DefaultAdvertisedEDNS
	}
	if c.TCPReadTimeout <= 0 {
		c.TCPReadTimeout = DefaultTCPReadTimeout
	}
	if c.TCPWriteTimeout <= 0 {
		c.TCPWriteTimeout = DefaultTCPWriteTimeout
	}
	if c.TCPIdleTimeout <= 0 {
		c.TCPIdleTimeout = DefaultTCPIdleTimeout
	}
	if c.TCPMaxAge <= 0 {
		c.TCPMaxAge = DefaultTCPMaxAge
	}
	if c.QueryTimeout <= 0 {
		c.QueryTimeout = DefaultQueryTimeout
	}
	if c.MaxTCPConns <= 0 {
		c.MaxTCPConns = DefaultMaxTCPConns
	}
	if c.MaxTCPPerIP <= 0 {
		c.MaxTCPPerIP = DefaultMaxTCPPerIP
	}
	if c.MaxInflight <= 0 {
		c.MaxInflight = DefaultMaxInflight
	}
	if c.MaxHold <= 0 {
		c.MaxHold = DefaultMaxHold
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = DefaultShutdownWait
	}
	return c
}
