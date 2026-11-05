package repeataccumulate

import (
	"sync"
	"testing"
)

var (
	mu   sync.Mutex
	runs int
)

// Expected: KindRepeat. Passes with -count=1, fails on round 2 with -count>=2.
// This is the cheapest probe: one command, one run.
func TestAccumulates(t *testing.T) {
	mu.Lock()
	runs++
	n := runs
	mu.Unlock()
	if n > 1 {
		t.Fatalf("state carried into round %d", n)
	}
}
