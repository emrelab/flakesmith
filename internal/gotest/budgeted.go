package gotest

import (
	"context"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/policy"
)

// Budgeted charges every run to a budget.
//
// Only Run is charged, not Discover: discovery is one build per package, it
// happens once, and without it there is nothing to spend a budget on. Counting
// it would make `--budget 1` refuse before the first test ever runs, which
// reads as a broken flag rather than a spent budget.
//
// Compose it inside the Cache (Cache{Budgeted{Exec}}), so a memoized result —
// a question already paid for — costs nothing.
type Budgeted struct {
	Inner  Runner
	Budget *policy.Budget
}

// Discover passes through to the inner runner.
func (b Budgeted) Discover(ctx context.Context, pkgDir string) ([]domain.TestID, error) {
	return b.Inner.Discover(ctx, pkgDir)
}

// Run reserves one run, then delegates.
//
// The reservation happens before the run, never during: killing a test halfway
// produces a truncated result that looks like a pass.
func (b Budgeted) Run(ctx context.Context, spec domain.RunSpec) (*domain.RunResult, error) {
	if err := b.Budget.Spend(1); err != nil {
		return nil, err
	}
	return b.Inner.Run(ctx, spec)
}

var _ Runner = Budgeted{}
