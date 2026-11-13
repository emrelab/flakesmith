package probe_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/gotest"
	"github.com/emrelab/go-flake-hunter/internal/probe"
)

// probeLeftovers counts witness files left in dir.
func probeLeftovers(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && domain.IsWitnessFileName(e.Name()) {
			n++
		}
	}
	return n, nil
}

// ConfirmRace must run the found witness ONCE with -race and write the label
// from that run's own outcome.
//
// This is the core of A1b: if -race were a search parameter, race overhead would
// push the timing witness below the verification threshold and the flake would
// never be found (measured: parallel-race fixture 1/3 with --race). Gathering
// evidence in a separate run preserves recall and makes the flag tell the truth.
func TestConfirmRaceLabelsEvidenceFromItsOwnRun(t *testing.T) {
	tgt := parallelTarget(t.TempDir())
	witness := &domain.Witness{
		Kind:   domain.KindParallel,
		Victim: tgt.Test,
		Flags:  domain.Flags{Parallel: 2},
		Steps: []domain.Step{
			{Label: domain.VictimLabel("TestAlpha"), Test: tgt.Test, WrapParallel: true},
		},
	}

	cases := []struct {
		name     string
		raceFail bool
		want     bool
		wantNote string
	}{
		{name: "reproduces under race", raceFail: true, want: true, wantNote: ""},
		{name: "does not reproduce under race", raceFail: false, want: false,
			wantNote: "not reproduced under a race run"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fail := tc.raceFail
			fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
				res := gotest.Result(s, nil)
				if fail {
					name := strings.Trim(s.RunRegex, "^$")
					res.Outcomes[domain.WitnessSubtest(name, "TestAlpha")] =
						domain.Outcome{Status: domain.StatusFail}
				}
				return res, nil
			}}

			race, note, err := probe.ConfirmRace(context.Background(), tgt, witness, fake)
			if err != nil {
				t.Fatalf("ConfirmRace: %v", err)
			}
			if fake.CallCount() != 1 {
				t.Errorf("a single run was expected, %d runs were made", fake.CallCount())
			}
			spec := fake.Runs()[0]
			if !spec.Race {
				t.Error("the run does not carry -race: the evidence is not race's")
			}
			if spec.Parallel != 2 {
				t.Errorf("parallel width %d, want 2: a serialised witness is a false green", spec.Parallel)
			}
			if race != tc.want {
				t.Errorf("race = %t, want %t", race, tc.want)
			}
			if note != tc.wantNote {
				t.Errorf("not = %q, want %q", note, tc.wantNote)
			}
			// The witness file must not stay on disk: if it leaks into later builds
			// the user gets an error about a test they never wrote.
			if leftovers, _ := probeLeftovers(tgt.Dir); leftovers != 0 {
				t.Errorf("%d witness files left on disk", leftovers)
			}
		})
	}
}

// Classification is race-independent: the same package yields a witness with
// Flags.Race=false whether -race was requested or not. The flag leaking into
// the search was exactly the regression that lowered recall; ConfirmRace writes it later.
func TestParallelWitnessCarriesNoRaceFlag(t *testing.T) {
	tgt := parallelTarget(t.TempDir())
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case s.RunRegex != "" && strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
			name := strings.Trim(s.RunRegex, "^$")
			res := gotest.Result(s, nil)
			if s.Parallel >= 2 {
				res.Outcomes[domain.WitnessSubtest(name, "TestAlpha")] = domain.Outcome{Status: domain.StatusFail}
			}
			return res, nil
		case s.RunRegex != "":
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
		case s.Parallel >= 2:
			return gotest.Result(s, map[string]domain.Status{
				"TestAlpha": domain.StatusFail, "TestBeta": domain.StatusPass,
			}), nil
		default:
			return gotest.Result(s, map[string]domain.Status{
				"TestAlpha": domain.StatusPass, "TestBeta": domain.StatusPass,
			}), nil
		}
	}}

	w, err := probe.Parallel{}.Probe(context.Background(), tgt, fake)
	if err != nil || w == nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if w.Flags.Race {
		t.Error("the search witness must not carry a race label: the evidence is not produced yet")
	}
}
