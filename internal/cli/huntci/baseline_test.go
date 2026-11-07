package clihuntci

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/clitest"
	"github.com/emrelab/flakesmith/internal/domain"
)

// The baseline's whole promise: a known flake does not break the gate, a new
// one does. Both in one test, because a lone "green" gate is ambiguous — it
// may mean a real flake was seen or that nothing at all was found.
func TestHuntBaselineGatesOnNewFlakesOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := clitest.FixtureDir(t, "order-basic")
	// The default baseline path is relative to the invoked dir: the test's cwd
	// must be temporary so nothing is written into the repo or testdata.
	work := t.TempDir()
	t.Chdir(work)
	store := filepath.Join(t.TempDir(), "c")

	code, _, stderr := clitest.Run("hunt", dir, "--capsule-dir", store, "--write-baseline")
	if code != domain.ExitOK {
		t.Fatalf("--write-baseline: code = %d (stderr=%s)", code, stderr)
	}
	path := filepath.Join(work, domain.CapsuleDir, capsule.BaselineFileName)
	b, err := capsule.LoadBaseline(path)
	if err != nil {
		t.Fatalf("baseline was not written (%s): %v", path, err)
	}
	if len(b.IDs) != 1 {
		t.Fatalf("baseline carries %d ids, expected 1: %v", len(b.IDs), b.IDs)
	}

	// 1) Known flake: the gate is green, but the flake is still reported.
	code, stdout, stderr := clitest.Run("hunt", dir, "--capsule-dir", store,
		"--baseline", path, "--fail-on-flake")
	if code != domain.ExitOK {
		t.Fatalf("known flake broke the gate: code = %d (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stderr, "known flake: "+b.IDs[0]) {
		t.Errorf("known flake was not reported:\n%s", stderr)
	}
	if !strings.Contains(stderr, "baseline: 1 known, 0 new") {
		t.Errorf("summary line missing:\n%s", stderr)
	}
	if !strings.Contains(stdout, "TestVictim") {
		t.Errorf("found flake report disappeared:\n%s", stdout)
	}

	// 2) An id outside the baseline: the gate is red (exit 1) and the flake is
	// not counted "known". A one-line baseline is enough here — what is really
	// measured is the direction of the id comparison.
	foreign := filepath.Join(t.TempDir(), "baseline.json")
	if err := capsule.WriteBaseline(foreign, []string{"fs_000000000000"}); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = clitest.Run("hunt", dir, "--capsule-dir", store,
		"--baseline", foreign, "--fail-on-flake")
	if code != domain.ExitFlakeFound {
		t.Fatalf("new flake did not break the gate: code = %d, want %d (stderr=%s)",
			code, domain.ExitFlakeFound, stderr)
	}
	if strings.Contains(stderr, "known flake") {
		t.Errorf("new flake counted as known:\n%s", stderr)
	}
}

// A missing baseline file is never guessed: if the user made a typo, a
// "clean" result would hide that nothing was compared.
func TestHuntMissingBaselineIsUsageError(t *testing.T) {
	dir := clitest.FixtureDir(t, "clean")
	code, _, stderr := clitest.Run("hunt", dir, "--baseline", filepath.Join(t.TempDir(), "missing.json"))
	if code != domain.ExitUsage {
		t.Fatalf("code = %d, want %d (stderr=%s)", code, domain.ExitUsage, stderr)
	}
	if !strings.Contains(stderr, "baseline not found") {
		t.Errorf("no explanation: %q", stderr)
	}
}
