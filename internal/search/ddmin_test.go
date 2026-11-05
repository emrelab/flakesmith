package search_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/search"
)

func ids(n int) []domain.TestID {
	out := make([]domain.TestID, n)
	for i := range out {
		out[i] = domain.TestID{Name: fmt.Sprintf("Test%02d", i)}
	}
	return out
}

// contains is the universe: only subsets containing the true polluter reproduce.
func contains(polluter string) search.Predicate {
	return func(ctx context.Context, subset []domain.TestID) (bool, error) {
		for _, t := range subset {
			if t.Name == polluter {
				return true, nil
			}
		}
		return false, nil
	}
}

func TestMinimizeFindsSinglePolluter(t *testing.T) {
	calls := 0
	pred := contains("Test07")
	counted := func(ctx context.Context, s []domain.TestID) (bool, error) {
		calls++
		return pred(ctx, s)
	}

	got, err := search.Minimize(context.Background(), ids(16), counted, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Test07" {
		t.Fatalf("the minimal set should be Test07, got %v", got)
	}
	// Budget: a naive scan costs 16, ddmin should be near-logarithmic.
	if calls > 20 {
		t.Errorf("%d oracle calls: too many for ddmin", calls)
	}
}

// If two different polluters give the same result, one should suffice (minimality).
func TestMinimizeDropsRedundantPolluters(t *testing.T) {
	pred := func(ctx context.Context, s []domain.TestID) (bool, error) {
		for _, t := range s {
			if t.Name == "Test01" || t.Name == "Test02" {
				return true, nil
			}
		}
		return false, nil
	}
	got, err := search.Minimize(context.Background(), ids(8), pred, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("the redundancy should be dropped, got %v", got)
	}
}

// If the set is already minimal, nothing should be dropped.
func TestMinimizeKeepsNecessaryPair(t *testing.T) {
	pred := func(ctx context.Context, s []domain.TestID) (bool, error) {
		has01, has02 := false, false
		for _, t := range s {
			if t.Name == "Test01" {
				has01 = true
			}
			if t.Name == "Test02" {
				has02 = true
			}
		}
		return has01 && has02, nil
	}
	got, err := search.Minimize(context.Background(), ids(6), pred, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("both polluters are required, got %v", got)
	}
}

// If the oracle lies, the result must be verified: no silently wrong witness.
//
// The setup is deterministic: the oracle tells the truth on every odd call,
// and lies on every even one. Verification runs 3 times so at least one says "no";
// an unverified witness must not be published.
func TestMinimizeRejectsUnstablePredicate(t *testing.T) {
	calls := 0
	unstable := func(ctx context.Context, s []domain.TestID) (bool, error) {
		calls++
		if calls%2 == 0 {
			return false, nil
		}
		return contains("Test03")(ctx, s)
	}
	_, err := search.Minimize(context.Background(), ids(8), unstable, 3)
	if !errors.Is(err, domain.ErrNotReproducible) {
		t.Fatalf("expected ErrNotReproducible, got %v", err)
	}
}

func TestMinimizeRejectsEmptyInput(t *testing.T) {
	if _, err := search.Minimize(context.Background(), nil, contains("x"), 1); err == nil {
		t.Fatal("an empty candidate list must be rejected")
	}
}

// Context cancellation must propagate during the search (required for budget/cancel).
func TestMinimizeRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	pred := func(ctx context.Context, s []domain.TestID) (bool, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		return contains("Test05")(ctx, s)
	}
	if _, err := search.Minimize(ctx, ids(16), pred, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
