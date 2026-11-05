package parallelrace

import (
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
const holdFor = 40 * time.Millisecond

var (
	mu        sync.Mutex
	active    int
	maxActive int
)

func enter(t *testing.T) {
	t.Helper()

	mu.Lock()
	active++
	if active > maxActive {
		maxActive = active
	}
	seen := maxActive
	mu.Unlock()

	time.Sleep(holdFor) // wait long enough for the other test to enter too

	mu.Lock()
	active--
	mu.Unlock()

	if seen > 1 {
		t.Fatalf("concurrent access: maxActive=%d", seen)
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
