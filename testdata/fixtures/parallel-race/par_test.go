package parallelrace

import (
	"flag"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Expected: KindParallel.
//
// Design note: a single test function never runs in parallel with a copy of
// itself (Go testing docs), so "incrementing test + parallel" is not enough.
// We use a barrier that fails when two DIFFERENT tests are active at once:
//
//	-parallel=1  → tests run in sequence, maxActive never sees 2 → PASS
//	-parallel>=2 → both inside at once → maxActive=2 → FAIL
//
// This is the simplest deterministic model of the "shared resource + concurrent
// access" flake.
//
// Load resistance: the original design held the window with a fixed 40ms
// sleep. On a loaded runner the second test can enter more than 40ms after
// the first, the window never overlaps, and the probe honestly concludes
// "no parallel flake" — the fixture, not the tool, flaked under load. The
// window is therefore an adaptive poll: while another test has not arrived,
// stay for up to overlapWait; once both are in, poll returns in milliseconds,
// so the failing (parallel) path stays fast. The sequential (-parallel=1)
// path pays overlapWait once per test — bounded and intentional.
// 2s, not the original 500ms: loaded windows runners (issue #12) can keep
// the sibling from entering the barrier for longer than 500ms — Defender
// scans every spawned test binary — and one squeezed verification round
// reads as "not deterministic". The sequential path pays this once per
// test; the failing (overlap) path still resolves in milliseconds.
const overlapWait = 2 * time.Second

var (
	mu        sync.Mutex
	active    int
	maxActive int
)

// parallelWidth reads the parallelism the test binary itself was started
// with (the -test.parallel flag Go always passes). -parallel=1 means no
// sibling can ever overlap, so the hold is pointless — this keeps the
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

func enter(t *testing.T) {
	t.Helper()

	mu.Lock()
	active++
	// seen = my concurrency at ENTRY. The victim is the test that walked in
	// while another was still active — the first arriver holds the door but
	// never fails. Exactly one victim per overlap, as before.
	seen := active
	if active > maxActive {
		maxActive = active
	}
	mu.Unlock()

	if seen == 1 && parallelWidth() > 1 {
		// First in: hold the door open for a late sibling.
		//
		// Only when the run can actually deliver a sibling: with
		// -parallel=1 no second test exists to arrive, and holding would
		// charge overlapWait to every sequential run — the shrinking loop
		// and the race confirmation run dozens of them, and the -race CI
		// step timed out on that arithmetic.
		deadline := time.Now().Add(overlapWait)
		for time.Now().Before(deadline) {
			mu.Lock()
			a := active
			mu.Unlock()
			if a > 1 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	active--
	mu.Unlock()

	if seen > 1 {
		t.Fatalf("concurrent access: activeAtEntry=%d", seen)
	}
}

func TestParallelAlpha(t *testing.T) {
	t.Parallel()
	enter(t)
}

func TestParallelBeta(t *testing.T) {
	t.Parallel()
	enter(t)
}
