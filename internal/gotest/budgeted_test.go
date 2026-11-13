package gotest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/gotest"
	"github.com/emrelab/go-flake-hunter/internal/policy"
)

// The budget counts runs only: discovery is a build paid once and without it
// there is nothing to spend the budget on. If it counted, `--budget 1` would
// refuse before any test ever ran.
func TestBudgetedChargesRunsNotDiscovery(t *testing.T) {
	inner := &gotest.Fake{}
	b := gotest.Budgeted{Inner: inner, Budget: policy.New(2, time.Minute)}
	spec := domain.RunSpec{Package: "p", Count: 1}

	for i := 0; i < 2; i++ {
		if _, err := b.Run(context.Background(), spec); err != nil {
			t.Fatalf("run %d must fit: %v", i+1, err)
		}
	}
	if _, err := b.Run(context.Background(), spec); !errors.Is(err, domain.ErrBudgetExceeded) {
		t.Fatalf("run 3 must be refused, got %v", err)
	}
	// A refused run must never reach the subprocess: running tests for nothing
	// destroys the meaning of the budget.
	if inner.CallCount() != 2 {
		t.Errorf("inner runner called %d times, want 2", inner.CallCount())
	}

	if _, err := b.Discover(context.Background(), "p"); err != nil {
		t.Fatalf("discovery must not draw from the budget: %v", err)
	}
}
