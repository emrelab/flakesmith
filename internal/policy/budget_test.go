package policy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/policy"
)

func TestBudgetCountsRunsToTheLimit(t *testing.T) {
	b := policy.New(2, time.Minute)
	if err := b.Spend(1); err != nil {
		t.Fatalf("run 1 must fit: %v", err)
	}
	if err := b.Spend(1); err != nil {
		t.Fatalf("run 2 must fit: %v", err)
	}
	err := b.Spend(1)
	if !errors.Is(err, domain.ErrBudgetExceeded) {
		t.Fatalf("run 3 must be refused, got %v", err)
	}
	if b.Runs() != 2 {
		t.Errorf("Runs = %d, a refused run must not be counted", b.Runs())
	}
	if b.Remaining() != 0 {
		t.Errorf("Remaining = %d", b.Remaining())
	}
}

// A refused budget must report the same number when retried: telling two probes
// "budget ended at 3/2" and "at 5/2" makes the report untrustworthy.
func TestBudgetRefusalIsSticky(t *testing.T) {
	b := policy.New(1, time.Minute)
	_ = b.Spend(1)
	first := b.Spend(1)
	second := b.Spend(1)
	if first == nil || second == nil {
		t.Fatal("both must be refused")
	}
	if first.Error() != second.Error() {
		t.Errorf("message changed: %q != %q", first, second)
	}
}

func TestBudgetWallClock(t *testing.T) {
	b := policy.New(100, time.Nanosecond)
	time.Sleep(time.Millisecond)
	if err := b.Spend(1); !errors.Is(err, domain.ErrBudgetExceeded) {
		t.Fatalf("the time budget must run out, got %v", err)
	}
}

// Zero/negative means "default": CLI flag values must be passable straight through.
func TestBudgetDefaultsOnZero(t *testing.T) {
	b := policy.New(0, 0)
	if b.MaxRuns != policy.DefaultMaxRuns || b.MaxWall != policy.DefaultMaxWall {
		t.Errorf("defaults not applied: %d, %s", b.MaxRuns, b.MaxWall)
	}
}

// A nil budget must behave like "unlimited": code paths without a hunt
// (library use) must not be forced to carry a budget.
func TestNilBudgetIsUnlimited(t *testing.T) {
	var b *policy.Budget
	for i := 0; i < 5; i++ {
		if err := b.Spend(1); err != nil {
			t.Fatalf("a nil budget must not refuse: %v", err)
		}
	}
	if b.Runs() != 0 || b.Remaining() != -1 {
		t.Errorf("a nil budget must not count: %d, %d", b.Runs(), b.Remaining())
	}
}
