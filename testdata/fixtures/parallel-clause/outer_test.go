package parallelclause_test

import (
	"testing"

	"fixture/parallelclause"
)

// Expected: ErrUnsupportedTest (exit 5).
//
// The victim lives in `parallelclause_test`; its only overlap partner,
// TestInnerAlpha, lives in `parallelclause`. The two clauses are linked into one
// test binary, so the overlap is real and the failure is real — but the witness
// file is generated into one clause and calls test functions by name, and a test
// function of the other clause is not addressable from there. Saying "no flake"
// would be a lie; saying "witness did not compile" would be a misdiagnosis.
//
//	-parallel=1  → PASS (serialised: nothing overlaps)
//	-parallel>=2 → FAIL on this test only (TestInnerAlpha passes)
//	alone        → PASS (a test broken on its own is not a flake)
func TestOuterVictim(t *testing.T) {
	t.Parallel()
	if seen, overlapped := parallelclause.Touch(t.Name()); overlapped {
		t.Fatalf("concurrent access: another test held the same resource (activeAtEntry=%d)", seen)
	}
}
