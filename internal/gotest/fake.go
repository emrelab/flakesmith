package gotest

import (
	"context"
	"sync"

	"github.com/emrelab/go-flake-hunter/internal/domain"
)

// Fake is the Runner used by every layer above gotest in tests.
//
// It exists because a test suite that depends on a real `go test` invocation is
// slow, needs a compiler, and can itself flake. Probe/search logic is verified
// against a scripted universe instead; only integration tests touch Exec.
type Fake struct {
	// DiscoverFn answers Discover. Nil -> no tests.
	DiscoverFn func(pkgDir string) ([]domain.TestID, error)
	// RunFn is the scripted universe: same spec in, same result out.
	RunFn func(spec domain.RunSpec) (*domain.RunResult, error)

	mu   sync.Mutex
	runs []domain.RunSpec // every spec that was requested, in order
}

// Discover implements Runner.
func (f *Fake) Discover(ctx context.Context, pkgDir string) ([]domain.TestID, error) {
	if f.DiscoverFn == nil {
		return nil, nil
	}
	return f.DiscoverFn(pkgDir)
}

// Run implements Runner and records the spec.
func (f *Fake) Run(ctx context.Context, spec domain.RunSpec) (*domain.RunResult, error) {
	f.mu.Lock()
	f.runs = append(f.runs, spec)
	f.mu.Unlock()

	if f.RunFn == nil {
		return &domain.RunResult{Spec: spec, Outcomes: map[string]domain.Outcome{}}, nil
	}
	return f.RunFn(spec)
}

// Runs returns the recorded specs (copy).
func (f *Fake) Runs() []domain.RunSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.RunSpec, len(f.runs))
	copy(out, f.runs)
	return out
}

// CallCount is the budget assertion most tests need.
func (f *Fake) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

// Result builds a RunResult from name->status pairs; a small convenience for
// scripting a universe.
func Result(spec domain.RunSpec, statuses map[string]domain.Status) *domain.RunResult {
	res := &domain.RunResult{Spec: spec, Outcomes: make(map[string]domain.Outcome, len(statuses))}
	for name, st := range statuses {
		res.Outcomes[name] = domain.Outcome{Status: st}
		if st != domain.StatusSkip {
			res.Order = append(res.Order, name)
		}
	}
	return res
}
