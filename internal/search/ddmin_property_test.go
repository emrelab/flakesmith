package search_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/search"
)

// hasAll is the interaction predicate: only subsets carrying every listed test
// reproduce the failure. Unlike a single-polluter `contains`, shrinking one
// member must never accidentally satisfy it, so it stresses ddmin's complement
// step in a way single-element targets do not.
func hasAll(names ...string) search.Predicate {
	return func(ctx context.Context, subset []domain.TestID) (bool, error) {
		for _, want := range names {
			found := false
			for _, id := range subset {
				if id.Name == want {
					found = true
					break
				}
			}
			if !found {
				return false, nil
			}
		}
		return true, nil
	}
}

// hasFloor is the size predicate: reproduction needs Test05 plus at least two
// companions. The 1-wise-minimal witness is any triple containing Test05, so
// ddmin may legitimately stop anywhere at size 3 — the assertions below must
// hold for whichever triple it picks.
func hasFloor() search.Predicate {
	base := hasAll("Test05")
	return func(ctx context.Context, subset []domain.TestID) (bool, error) {
		if len(subset) < 3 {
			return false, nil
		}
		return base(ctx, subset)
	}
}

// isSubset reports whether every element of got appears in want (by String).
// ddmin only ever removes candidates, so this must never fail; a hit means the
// shrink invented a test that was never in the candidate list.
func isSubset(got, want []domain.TestID) bool {
	pool := make(map[string]int, len(want))
	for _, id := range want {
		pool[id.String()]++
	}
	for _, id := range got {
		if pool[id.String()] == 0 {
			return false
		}
		pool[id.String()]--
	}
	return true
}

func sameSet(a, b []domain.TestID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMinimizeProperties asserts the three invariants Minimize promises for
// every monotone predicate, across several input sizes:
//
//  1. the shrunk witness still satisfies the predicate (no false witness),
//  2. the witness is a subset of the candidates (nothing invented),
//  3. re-shrinking the witness changes nothing (idempotence — a ddmin result
//     is 1-wise minimal, so no proper subset of it can reproduce).
//
// Deterministic by construction: the predicates are pure, no rand involved.
func TestMinimizeProperties(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		pred search.Predicate
	}{
		{"single polluter", contains("Test07")},
		// Names must exist even in the smallest candidate set (n=8), or the
		// input itself fails the predicate and Minimize correctly refuses.
		{"necessary pair", hasAll("Test02", "Test06")},
		{"either-or", func(ctx context.Context, s []domain.TestID) (bool, error) {
			a, err := contains("Test01")(ctx, s)
			if err != nil {
				return false, err
			}
			b, err := contains("Test03")(ctx, s)
			return a || b, err
		}},
		{"size floor 3", hasFloor()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{8, 16, 32} {
				t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
					input := ids(n)
					got, err := search.Minimize(ctx, input, tc.pred, 2)
					if err != nil {
						t.Fatalf("Minimize(%d candidates): %v", n, err)
					}
					// 1: the witness reproduces.
					ok, err := tc.pred(ctx, got)
					if err != nil {
						t.Fatal(err)
					}
					if !ok {
						t.Errorf("witness %v does not satisfy the predicate", got)
					}
					// 2: nothing was invented.
					if !isSubset(got, input) {
						t.Errorf("witness %v is not a subset of %d candidates", got, n)
					}
					// 3: shrinking an already-minimal set is a no-op.
					again, err := search.Minimize(ctx, got, tc.pred, 2)
					if err != nil {
						t.Fatalf("re-Minimize(%v): %v", got, err)
					}
					if !sameSet(got, again) {
						t.Errorf("not idempotent: %v then %v", got, again)
					}
				})
			}
		})
	}
}

// TestMinimizeWitnessIsOneWiseMinimal spot-checks the property that makes the
// idempotence above structural: dropping any single element from the witness
// must break a monotone predicate.
func TestMinimizeWitnessIsOneWiseMinimal(t *testing.T) {
	ctx := context.Background()
	input := ids(24)
	pred := hasAll("Test04", "Test19")

	got, err := search.Minimize(ctx, input, pred, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("witness = %v, want the necessary pair only", got)
	}
	for i := range got {
		trimmed := append(append([]domain.TestID(nil), got[:i]...), got[i+1:]...)
		ok, err := pred(ctx, trimmed)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Errorf("dropping %s still reproduces: witness is not minimal", got[i].Name)
		}
	}
	if s := strings.Join(namesOf(got), ","); s == "" {
		t.Error("empty witness")
	}
}

func namesOf(xs []domain.TestID) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.Name
	}
	return out
}
