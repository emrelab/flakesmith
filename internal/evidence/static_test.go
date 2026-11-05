package evidence_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/evidence"
	"github.com/emrelab/flakesmith/internal/facts"
)

func scan(t *testing.T, fixture string) facts.PkgFacts {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", fixture))
	if err != nil {
		t.Fatal(err)
	}
	f, err := facts.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func witness(victim string, polluters ...string) *domain.Witness {
	w := &domain.Witness{
		Kind:   domain.KindOrder,
		Victim: domain.TestID{Package: "p", Name: victim},
	}
	for _, p := range polluters {
		w.Polluters = append(w.Polluters, domain.TestID{Package: "p", Name: p})
	}
	return w
}

// A shared global: the single most valuable line of evidence.
func TestBuildRecordsSharedGlobal(t *testing.T) {
	f := scan(t, "order-basic")
	ev := evidence.Build(witness("TestVictim", "TestPolluter"), f)

	if got := ev.GlobalsWritten["TestPolluter"]; len(got) == 0 || got[0] != "seeded" {
		t.Errorf("GlobalsWritten = %v", got)
	}
	if got := ev.GlobalsRead["TestVictim"]; len(got) == 0 || got[0] != "seeded" {
		t.Errorf("GlobalsRead = %v", got)
	}
	shared := evidence.SharedGlobals(f)
	if len(shared) == 0 || shared[0].Var != "seeded" ||
		shared[0].Writer != "TestPolluter" || shared[0].Reader != "TestVictim" {
		t.Errorf("SharedGlobals = %+v", shared)
	}
}

// Touching disk is visible statically: the call name is recorded, not the path.
func TestBuildRecordsFileCalls(t *testing.T) {
	f := scan(t, "file-leak")
	ev := evidence.Build(witness("TestVictimExpectsNoFile", "TestPolluterWritesFile"), f)

	if len(ev.FilesTouched) != 1 {
		t.Fatalf("FilesTouched = %v", ev.FilesTouched)
	}
	if !strings.Contains(ev.FilesTouched[0], "TestPolluterWritesFile") ||
		!strings.Contains(ev.FilesTouched[0], "os.WriteFile") {
		t.Errorf("evidence line missing: %q", ev.FilesTouched[0])
	}
}

// What the victim itself writes is not pollution evidence: only polluters
// count, so every capsule does not point at the same fake cause.
func TestBuildIgnoresVictimEffects(t *testing.T) {
	f := scan(t, "file-leak")
	ev := evidence.Build(witness("TestPolluterWritesFile"), f)
	if len(ev.FilesTouched) != 0 {
		t.Errorf("no evidence should be produced without a polluter: %v", ev.FilesTouched)
	}
	if len(ev.GlobalsWritten) != 0 {
		t.Errorf("no global evidence should be produced without a polluter: %v", ev.GlobalsWritten)
	}
}

// Env keys come from the witness; never the values.
func TestBuildCarriesEnvKeysOnly(t *testing.T) {
	ev := evidence.Build(&domain.Witness{
		Kind:    domain.KindEnv,
		Victim:  domain.TestID{Package: "p", Name: "TestDependsOnEnv"},
		EnvDiff: []string{"FIXTURE_MODE"},
	}, facts.PkgFacts{})
	if len(ev.EnvDiff) != 1 || ev.EnvDiff[0] != "FIXTURE_MODE" {
		t.Errorf("EnvDiff = %v", ev.EnvDiff)
	}
}
