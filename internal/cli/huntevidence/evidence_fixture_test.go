package clihuntevidence

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emrelab/flakesmith/internal/clitest"
)

// The parallel-race fixture is the instrument every concurrency claim is
// calibrated against, so its own contract gets asserted directly, not only
// through `hunt`:
//
//	-parallel=1                    → PASS (the class is concurrency, not order)
//	every other test present       → FAIL, always on the victim
//	victim alone                   → PASS (a broken-on-its-own test is not a flake)
//	and the fallback deadline is never paid when a sibling was selected.
//
// The last clause is the regression guard for issue #12: the previous design
// polled a counter for a fixed window, missed the sibling's sub-microsecond
// critical section, and paid the whole window on every witness run — which is
// what pushed the -race CI step into its timeout. Its failure mode (a squeezed
// window reads as "no flake") is invisible to a pass/fail assertion, so the
// fixture logs when the deadline expires and we assert on that line.
func TestParallelRaceFixtureContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := clitest.FixtureDir(t, "parallel-race")

	goTest := func(t *testing.T, args ...string) (pass bool, out string) {
		t.Helper()
		cmd := exec.Command("go", append([]string{"test", "-count=1"}, args...)...)
		cmd.Dir = dir
		// Keep the fixture's runs off the network and out of the user's env.
		cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
		raw, err := cmd.CombinedOutput()
		out = string(raw)
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				t.Fatalf("go test did not run: %v\n%s", err, out)
			}
			return false, out
		}
		return true, out
	}
	assertNoDeadline := func(t *testing.T, out string) {
		t.Helper()
		if strings.Contains(out, "overlap-window expired") {
			t.Errorf("the fallback deadline was paid although a sibling was selected; "+
				"the overlap must be signalled, not polled:\n%s", out)
		}
	}

	t.Run("serial passes", func(t *testing.T) {
		pass, out := goTest(t, "-parallel=1", "-v")
		if !pass {
			t.Fatalf("-parallel=1 must pass, the flake is concurrency:\n%s", out)
		}
		assertNoDeadline(t, out)
	})

	t.Run("overlap fails on the victim only", func(t *testing.T) {
		// Repeated so a scheduling coincidence cannot pass this by luck.
		for i := 0; i < 3; i++ {
			pass, out := goTest(t, "-parallel=2", "-v")
			if pass {
				t.Fatalf("run %d: -parallel=2 must fail:\n%s", i+1, out)
			}
			if !strings.Contains(out, "--- FAIL: TestParallelBeta") {
				t.Errorf("run %d: the victim must be the one that fails:\n%s", i+1, out)
			}
			if strings.Contains(out, "--- FAIL: TestParallelAlpha") {
				t.Errorf("run %d: the polluter is innocent and must pass:\n%s", i+1, out)
			}
			assertNoDeadline(t, out)
		}
	})

	t.Run("victim alone passes", func(t *testing.T) {
		// The tool's solo check depends on this: a test that fails by itself is
		// broken, not a flake, and must never be published as one.
		pass, out := goTest(t, "-run", "^TestParallelBeta$", "-v")
		if !pass {
			t.Fatalf("the victim alone must pass:\n%s", out)
		}
		assertNoDeadline(t, out)
	})

	t.Run("no fixed wait when no sibling exists", func(t *testing.T) {
		// The property the -race CI timeout needs: a run that cannot contain a
		// sibling must not hold for a window. Measured, not guessed — a parked
		// goroutine is invisible to pass/fail, only to the clock.
		//
		// Relative, not absolute: build/startup cost is paid by both sides and
		// cancels out, so a loaded runner does not make this flaky. best-of-3
		// on each side absorbs a single scheduling hiccup.
		goTest(t, "-parallel=1") // warm the build cache
		best := func(t *testing.T, args ...string) time.Duration {
			t.Helper()
			fastest := time.Duration(1<<62 - 1)
			for i := 0; i < 3; i++ {
				cmd := exec.Command("go", append([]string{"test", "-count=1"}, args...)...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
				start := time.Now()
				_ = cmd.Run()
				if d := time.Since(start); d < fastest {
					fastest = d
				}
			}
			return fastest
		}
		serial := best(t, "-parallel=1")
		solo := best(t, "-run", "^TestParallelBeta$")
		// The old design cost a fixed 2s here; 1s of slack above the serial
		// baseline leaves room for noise and none for a re-introduced wait.
		if solo > serial+time.Second {
			t.Errorf("a solo run costs %v against %v serial: the fixture waits for a "+
				"sibling that this run cannot select", solo, serial)
		}
	})

	t.Run("no witness file is left behind", func(t *testing.T) {
		left, err := filepath.Glob(filepath.Join(dir, "fs_witness*"))
		if err != nil {
			t.Fatal(err)
		}
		if len(left) != 0 {
			t.Errorf("witness files left in the fixture: %v", left)
		}
	})
}
