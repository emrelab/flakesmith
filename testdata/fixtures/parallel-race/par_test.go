package parallelrace

import (
	"flag"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Expected: KindParallel.
//
// Design note: a single test function never runs in parallel with a copy of
// itself (Go testing docs), so "incrementing test + parallel" is not enough.
// We use a barrier that observes two DIFFERENT tests being inside at once:
//
//	-parallel=1  → tests run in sequence, the overlap never happens → PASS
//	-parallel>=2 → both inside at once → the VICTIM fails
//
// TestParallelBeta is the victim: it is the test with the bug (it reads state
// another test may be holding). TestParallelAlpha is the innocent polluter —
// it shares the resource and passes. That asymmetry is the point: a fixture
// where both failed would hand the hunter two capsules and no
// polluter→victim direction.
//
// Load resistance (issue #12). The previous version held a window open by
// polling a counter every 5ms for a fixed 2s, and whichever test entered
// second failed. Three defects came out of that:
//
//  1. The poll could not see a sibling that entered and left between two
//     samples — the sibling's critical section is sub-microsecond — so the
//     holder always burned the full window on the happy path, and every
//     witness run paid 2s (that arithmetic is what timed out the -race CI
//     step).
//  2. Which test failed depended on which goroutine reached the barrier first.
//     The witness is supposed to fail on its victim specifically, so a
//     reordered arrival read as "the flake did not reproduce".
//  3. If a loaded runner (windows + Defender) kept the sibling from being
//     scheduled inside the fixed window, no overlap was ever observed and the
//     run passed — the tool then honestly refused the witness ("verification
//     2/3 failed") and the fixture, not the tool, flaked.
//
// Now the overlap is a channel, not a sample: the second arriver closes it and
// the first wakes in microseconds. The deadline is only a fallback for "no
// sibling is part of this run" and is skipped when the run's -test.run cannot
// select a sibling, so the tool never pays it. The victim role is fixed by
// name, so arrival order no longer decides who fails.
const overlapWait = 30 * time.Second

var (
	mu     sync.Mutex
	active int
	once   sync.Once
	// inUse is closed exactly once, the first time two tests are inside enter
	// at the same time. Package-level because the whole point is shared state
	// between tests — that is the bug being modelled.
	inUse = make(chan struct{})
)

// parallelWidth reads the parallelism the test binary itself was started
// with (the -test.parallel flag Go always passes). -parallel=1 means no
// sibling can ever overlap, so the wait is pointless — this keeps the
// sequential probe path at its old, free cost.
func parallelWidth() int {
	f := flag.Lookup("test.parallel")
	if f == nil {
		return 1
	}
	n, err := strconv.Atoi(f.Value.String())
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// canOverlap reports whether another test could still be scheduled beside this
// one, so waiting for it is not pure loss.
//
// Two shapes occur:
//
//   - a witness step: the run regex is the generated witness function and the
//     sibling is a subtest of it. The observer's name carries its parents
//     ("TestFlakeWitness/victim/TestParallelBeta") and only the parent knows
//     what it registered, so we wait.
//   - a plain top-level run: -test.run decides, and the regex is checked
//     against the only other test in this package. `-run ^TestParallelBeta$`
//     cannot produce a sibling, so it must not pay the deadline; that run
//     happens ~20 times per hunt (solo and verification passes).
func canOverlap(t *testing.T) bool {
	if parallelWidth() < 2 {
		return false
	}
	name := t.Name()
	if strings.Contains(name, "/") {
		return true
	}
	run := flag.Lookup("test.run")
	if run == nil {
		return true
	}
	// -run splits on "/"; the first element selects top-level tests.
	first, _, _ := strings.Cut(run.Value.String(), "/")
	rx, err := regexp.Compile(first)
	if err != nil {
		return true // Not this fixture's call to judge: wait rather than risk a false pass.
	}
	for _, other := range []string{"TestParallelAlpha", "TestParallelBeta"} {
		if other != name && rx.MatchString(other) {
			return true
		}
	}
	return false
}

// enter models "touch the shared resource". The victim fails only when it can
// see a sibling inside at the same time; the polluter never fails.
func enter(t *testing.T, victim bool) {
	t.Helper()

	mu.Lock()
	active++
	seen := active
	if seen > 1 {
		once.Do(func() { close(inUse) })
	}
	mu.Unlock()

	overlapped := seen > 1
	if !overlapped && canOverlap(t) {
		// Hold the resource until a sibling shows up (instant) or until the
		// fallback deadline proves none was selected for this run.
		timer := time.NewTimer(overlapWait)
		select {
		case <-inUse:
			overlapped = true
		case <-timer.C:
			// Nobody came. That is only legitimate when this run really has no
			// sibling; the contract test asserts this line never shows up for
			// runs that do (see internal/cli/fixture_parallel_race_test.go).
			t.Log("overlap-window expired: no sibling arrived")
		}
		timer.Stop()
	}

	mu.Lock()
	active--
	mu.Unlock()

	if victim && overlapped {
		t.Fatalf("concurrent access: another test held the same resource (activeAtEntry=%d)", seen)
	}
}

func TestParallelAlpha(t *testing.T) {
	t.Parallel()
	enter(t, false)
}

func TestParallelBeta(t *testing.T) {
	t.Parallel()
	enter(t, true)
}
