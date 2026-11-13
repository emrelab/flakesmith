package search

import (
	"context"
	"fmt"

	"github.com/emrelab/go-flake-hunter/internal/domain"
)

// Predicate answers "does this subset still reproduce the failure?".
//
// It is allowed to be expensive (it runs a build) and it is allowed to lie
// occasionally — Minimize re-verifies, so a single wrong answer cannot become
// a published witness.
type Predicate func(ctx context.Context, subset []domain.TestID) (bool, error)

// Minimize returns the smallest subset the predicate still accepts, and
// confirms it by re-running the predicate verify times.
//
// verify < 1 is coerced to 1: publishing an unverified witness is the one
// outcome worse than finding nothing.
func Minimize(ctx context.Context, candidates []domain.TestID, pred Predicate, verify int) ([]domain.TestID, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no candidates", domain.ErrInternal)
	}
	if verify < 1 {
		verify = 1
	}

	min, err := ddmin(ctx, candidates, pred)
	if err != nil {
		return nil, err
	}

	failures, firstMiss := 0, 0
	for i := 0; i < verify; i++ {
		ok, err := pred(domain.FreshRuns(ctx), min)
		if err != nil {
			return nil, err
		}
		if ok {
			failures++
		} else if firstMiss == 0 {
			firstMiss = i + 1
		}
	}
	if failures != verify {
		return min, &VerificationError{Observation: domain.Observe(verify, failures), FirstMiss: firstMiss}
	}
	return min, nil
}

// ddmin is the classic Zeller delta-debugging loop.
//
// n starts at 2 and doubles only when neither a subset nor its complement
// reduces; every accepted reduction resets n to 2 because the problem got
// smaller. Complexity is O(n²) predicate calls worst case, O(n log n) typical —
// for the ≤ 50 candidates the order probe considers, that is tens of builds.
func ddmin(ctx context.Context, candidates []domain.TestID, pred Predicate) ([]domain.TestID, error) {
	cur := append([]domain.TestID(nil), candidates...)
	n := 2

	for len(cur) >= 2 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunks := split(cur, n)
		reduced := false

		// 1) Is any single chunk enough?
		for _, c := range chunks {
			ok, err := pred(ctx, c)
			if err != nil {
				return nil, err
			}
			if ok {
				cur, n, reduced = c, 2, true
				break
			}
		}
		if reduced {
			continue
		}

		// 2) Is dropping one chunk still enough?
		for i := range chunks {
			comp := complement(chunks, i)
			if len(comp) == 0 {
				continue
			}
			ok, err := pred(ctx, comp)
			if err != nil {
				return nil, err
			}
			if ok {
				cur, reduced = comp, true
				if n > 2 {
					n--
				}
				break
			}
		}
		if reduced {
			continue
		}

		// 3) Neither worked: refine the granularity.
		if n >= len(cur) {
			break
		}
		n = min(n*2, len(cur))
	}
	return cur, nil
}

// split divides xs into at most n contiguous chunks.
func split(xs []domain.TestID, n int) [][]domain.TestID {
	if n > len(xs) {
		n = len(xs)
	}
	out := make([][]domain.TestID, 0, n)
	size := (len(xs) + n - 1) / n
	for i := 0; i < len(xs); i += size {
		end := min(i+size, len(xs))
		out = append(out, xs[i:end])
	}
	return out
}

// complement returns every chunk except the i-th, concatenated.
func complement(chunks [][]domain.TestID, i int) []domain.TestID {
	var out []domain.TestID
	for j, c := range chunks {
		if j == i {
			continue
		}
		out = append(out, c...)
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// VerificationError retains observations without weakening the strict confirmation gate.
type VerificationError struct {
	Observation domain.Observation
	FirstMiss   int
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("%s: verification %d/%d failed after shrinking (%d/%d reproduced)", domain.ErrNotReproducible, e.FirstMiss, e.Observation.Runs, e.Observation.Failures, e.Observation.Runs)
}
func (e *VerificationError) Unwrap() error { return domain.ErrNotReproducible }
