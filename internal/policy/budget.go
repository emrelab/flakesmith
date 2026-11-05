package policy

import (
	"fmt"
	"sync"
	"time"

	"github.com/emrelab/flakesmith/internal/domain"
)

// Defaults.
//
// Runs are cheap and wall clock is not: a clean test costs one invocation, but a
// real package has dozens of them and `./...` has dozens of packages. Measured
// on spf13/pflag (≈120 tests): a full scan needs roughly 300 runs, so a 200-run
// cap stopped the hunt mid-package and reported exit 4 — a correct signal, but
// the wrong default for "first 5 minutes". 1000 runs ≈ 3 minutes of warm builds
// and still bounds a pathological search; 5 minutes is the wall clock a CI job
// can afford to lose before someone notices.
const (
	DefaultMaxRuns = 1000
	DefaultMaxWall = 5 * time.Minute
)

// Budget caps how much a hunt may spend.
//
// Spend is called at the START of every run, never during one: cutting a test
// in half produces a wrong negative ("it passed") that looks like a clean
// result. Overshooting by one run's own timeout is bounded and deliberate.
//
// The zero value is unusable; construct with New.
type Budget struct {
	MaxRuns int
	MaxWall time.Duration

	mu      sync.Mutex
	runs    int
	started time.Time
	spent   error // first refusal; later calls repeat it verbatim
}

// New returns a budget. Zero or negative arguments mean "use the default", so
// callers can pass flag values straight through.
func New(maxRuns int, maxWall time.Duration) *Budget {
	if maxRuns <= 0 {
		maxRuns = DefaultMaxRuns
	}
	if maxWall <= 0 {
		maxWall = DefaultMaxWall
	}
	return &Budget{MaxRuns: maxRuns, MaxWall: maxWall, started: time.Now()}
}

// Spend reserves n runs, or reports why it cannot.
//
// The error is sticky: once a hunt is over budget, every later probe sees the
// same refusal instead of a fresh count, so the report cannot claim the budget
// was hit at two different run numbers.
func (b *Budget) Spend(n int) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.spent != nil {
		return b.spent
	}
	if b.started.IsZero() {
		b.started = time.Now()
	}
	if b.MaxRuns > 0 && b.runs+n > b.MaxRuns {
		b.spent = fmt.Errorf("%w: %d/%d runs (the next run did not fit)", domain.ErrBudgetExceeded, b.runs, b.MaxRuns)
		return b.spent
	}
	if b.MaxWall > 0 {
		if elapsed := time.Since(b.started); elapsed > b.MaxWall {
			b.spent = fmt.Errorf("%w: %s / %s", domain.ErrBudgetExceeded,
				elapsed.Round(time.Second), b.MaxWall)
			return b.spent
		}
	}
	b.runs += n
	return nil
}

// Runs is how many runs were spent so far.
func (b *Budget) Runs() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.runs
}

// Remaining is how many runs are left; -1 means unlimited.
func (b *Budget) Remaining() int {
	if b == nil || b.MaxRuns <= 0 {
		return -1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	n := b.MaxRuns - b.runs
	if n < 0 {
		return 0
	}
	return n
}
