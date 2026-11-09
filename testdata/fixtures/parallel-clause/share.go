// Package parallelclause models a concurrency flake whose polluter lives in the
// OTHER package clause: the test that holds the shared resource is in
// `parallelclause`, the victim is in `parallelclause_test`.
//
// The two clauses are two visibility boundaries inside ONE test binary, so their
// tests really do overlap under -parallel>=2. No generated witness can express
// that: the witness file is written into the victim's clause and calls test
// functions by name, and TestInnerAlpha is not addressable from there. Expected
// verdict: ErrUnsupportedTest. See README.md.
package parallelclause

import (
	"flag"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OverlapWait is the fallback deadline: only paid when this run cannot schedule
// a sibling at all, which the tests rule out by checking CanOverlap first. A
// barrier channel is what carries the signal; the deadline exists so a run with
// no sibling (the tool's solo pass, `-run ^TestOuterVictim$`) never hangs.
const OverlapWait = 30 * time.Second

var (
	mu     sync.Mutex
	active int
	once   sync.Once
	// inUse is closed exactly once, the first time two tests are inside Touch
	// at the same time. Package-level on purpose: shared state across two
	// clauses is exactly the bug being modelled.
	inUse = make(chan struct{})
)

// CanOverlap reports whether this run could still schedule another test beside
// the one named.
//
// -parallel=1 means no sibling can ever overlap, so waiting is pure loss. So is
// a -test.run that cannot select the sibling: the tool runs the victim alone
// about twenty times per hunt, and every one of those runs must stay free.
func CanOverlap(name string) bool {
	if parallelWidth() < 2 {
		return false
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
	for _, other := range []string{"TestInnerAlpha", "TestOuterVictim"} {
		if other != name && rx.MatchString(other) {
			return true
		}
	}
	return false
}

// parallelWidth reads the parallelism the test binary was started with (the
// -test.parallel flag Go always passes).
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

// Touch models "use the shared resource" and reports how many tests were inside
// at the same time, plus whether this call saw an overlap.
//
// Arrival order never decides the outcome: the caller names the victim, so the
// same test fails whichever goroutine reaches the barrier first.
func Touch(name string) (seen int, overlapped bool) {
	mu.Lock()
	active++
	seen = active
	if seen > 1 {
		once.Do(func() { close(inUse) })
	}
	mu.Unlock()

	overlapped = seen > 1
	if !overlapped && CanOverlap(name) {
		timer := time.NewTimer(OverlapWait)
		select {
		case <-inUse:
			overlapped = true
		case <-timer.C:
			// Nobody came: only legitimate when this run really has no
			// sibling, which CanOverlap already checked.
		}
		timer.Stop()
	}

	mu.Lock()
	active--
	mu.Unlock()
	return seen, overlapped
}
