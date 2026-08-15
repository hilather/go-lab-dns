package app

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

const (
	// defaultIdempotencyMax is the LRU cap when Options.IdempotencyMax is
	// unset or non-positive. Zero is not unlimited.
	defaultIdempotencyMax = 256
	// defaultAuditMax is the ring size when Options.AuditMax is unset or
	// non-positive. Zero is not unlimited.
	defaultAuditMax = 128
	// defaultPageLimit is used when Page.Limit is unset or non-positive.
	// Zero is not "return everything".
	defaultPageLimit = 100
	maxPageLimit     = 1000
)

// Options constructs an App. Store may already hold a bootstrap snapshot.
type Options struct {
	Store          *snapshot.Store
	Cache          *cache.Cache
	Health         *forwarder.Health
	Clock          testutil.Clock
	BootstrapPath  string
	IdempotencyMax int
	AuditMax       int
}

// App is the process-local Service implementation.
type App struct {
	mu            sync.Mutex
	store         *snapshot.Store
	cache         *cache.Cache
	health        *forwarder.Health
	clock         testutil.Clock
	bootstrapPath string
	idemp         *idempCache
	audit         *auditRing
	// afterCompile runs after a successful compile, before Swap. Tests use
	// it to interleave emergency disable with apply.
	afterCompile func()
}

var _ Service = (*App)(nil)

// New returns an App. A nil Store becomes an empty snapshot.Store.
func New(opts Options) *App {
	if opts.Store == nil {
		opts.Store = snapshot.NewStore()
	}
	if opts.Clock == nil {
		opts.Clock = testutil.SystemClock{}
	}
	// Non-positive caps would otherwise grow without bound.
	idempMax := opts.IdempotencyMax
	if idempMax <= 0 {
		idempMax = defaultIdempotencyMax
	}
	auditMax := opts.AuditMax
	if auditMax <= 0 {
		auditMax = defaultAuditMax
	}
	return &App{
		store:         opts.Store,
		cache:         opts.Cache,
		health:        opts.Health,
		clock:         opts.Clock,
		bootstrapPath: opts.BootstrapPath,
		idemp:         newIdempCache(idempMax),
		audit:         newAuditRing(auditMax),
	}
}

// Store returns the snapshot store. DNS still loads from this pointer once
// per query; App does not wrap the data plane.
func (s *App) Store() *snapshot.Store {
	if s == nil {
		return nil
	}
	return s.store
}

func (s *App) requireCtx(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func (s *App) active() (*snapshot.Snapshot, error) {
	if s == nil || s.store == nil {
		return nil, domainerr.Internal("no snapshot store")
	}
	snap := s.store.Load()
	if snap == nil {
		return nil, domainerr.Internal("no active snapshot")
	}
	return snap, nil
}

func drifted(snap *snapshot.Snapshot) bool {
	if snap == nil {
		return false
	}
	return snap.Revision != snap.BootstrapRevision
}

func revisionView(snap *snapshot.Snapshot) RevisionView {
	if snap == nil {
		return RevisionView{}
	}
	return RevisionView{
		BootstrapRevision: snap.BootstrapRevision,
		RuntimeRevision:   snap.Revision,
		Generation:        snap.Generation,
		Drifted:           drifted(snap),
		LoadedAt:          snap.CompiledAt,
	}
}

func cloneState(st *model.State) (*model.State, error) {
	if st == nil {
		return nil, domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: "required", Message: "state is nil"})
	}
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.Internal("clone marshal: " + err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.Internal("clone unmarshal: " + err.Error())
	}
	return &out, nil
}

func compileCandidate(ctx context.Context, st *model.State, prev *snapshot.Snapshot, clock testutil.Clock, emergencyOff bool) (*snapshot.Snapshot, error) {
	opts := compiler.CompileOpts{
		Clock:             clock,
		EmergencyChaosOff: emergencyOff,
	}
	if prev != nil {
		opts.BootstrapRevision = prev.BootstrapRevision
		opts.Generation = prev.Generation + 1
		if opts.BootstrapRevision == "" {
			opts.BootstrapRevision = prev.Revision
		}
	}
	snap, err := compiler.Compile(ctx, st, opts)
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.Internal(err.Error())
}

func pageLimit(p Page) int {
	if p.Limit <= 0 {
		return defaultPageLimit
	}
	if p.Limit > maxPageLimit {
		return maxPageLimit
	}
	return p.Limit
}
