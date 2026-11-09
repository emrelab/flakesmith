package parallelclause

import "testing"

// Expected: the polluter. It shares the resource, calls t.Parallel() and always
// passes; the victim is TestOuterVictim in the other clause.
//
// This test is deliberately NOT the victim: a fixture where both failed would
// hand the hunter two capsules and no polluter->victim direction.
func TestInnerAlpha(t *testing.T) {
	t.Parallel()
	_, _ = Touch(t.Name())
}
