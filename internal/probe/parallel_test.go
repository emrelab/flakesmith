package probe_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/facts"
	"github.com/emrelab/flakesmith/internal/gotest"
	"github.com/emrelab/flakesmith/internal/probe"
)

func parallelTarget(dir string) probe.Target {
	return probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestAlpha"},
		Dir:  dir,
		Facts: facts.PkgFacts{
			Clause: map[string]string{
				"TestAlpha": "p",
				"TestBeta":  "p",
			},
			// Static gate: without t.Parallel() in the package the class is impossible.
			TestParallel: map[string]bool{"TestAlpha": true, "TestBeta": true},
		},
	}
}

// Class separation: a flake that fails in a serial run is not parallel, it is order's job.
// Two probes must not claim the same capsule with different witnesses.
func TestParallelDefersToOrderWhenSerialFails(t *testing.T) {
	tgt := parallelTarget(t.TempDir())
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if s.RunRegex != "" {
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
		}
		return gotest.Result(s, map[string]domain.Status{
			"TestAlpha": domain.StatusFail, "TestBeta": domain.StatusPass,
		}), nil
	}}

	_, err := probe.Parallel{}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrNotReproduced) {
		t.Fatalf("must not step on order's job, got %v", err)
	}
}

// A package that fails only at width >= 2: the parallel class must be found and
// the witness must carry enough width — a replay with 1 proves nothing.
func TestParallelFindsConcurrencyFlake(t *testing.T) {
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
	if err != nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if w.Kind != domain.KindParallel {
		t.Errorf("Kind = %s", w.Kind)
	}
	if len(w.Polluters) != 1 || w.Polluters[0].Name != "TestBeta" {
		t.Errorf("polluter = %+v", w.Polluters)
	}
	if w.Flags.Parallel < len(w.Steps) {
		t.Errorf("parallel width %d, step count %d: the witness serialises", w.Flags.Parallel, len(w.Steps))
	}
}

// A single-test package has nothing to overlap with: the probe must say
// "does not explain" and must not fabricate a witness.
func TestParallelSingleTestPackageIsNotAFlake(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestAlpha"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{
			Clause:       map[string]string{"TestAlpha": "p"},
			TestParallel: map[string]bool{"TestAlpha": true},
		},
	}
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if s.RunRegex != "" && s.Parallel == 1 {
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
		}
		if s.RunRegex != "" {
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
		}
		if s.Parallel >= 2 {
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusFail}), nil
		}
		return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
	}}

	_, err := probe.Parallel{}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrNotReproduced) {
		t.Fatalf("expected ErrNotReproduced, got %v", err)
	}
}

// TestMain never goes into a witness: its signature is *testing.M, calling it does not compile.
func TestParallelNeverCallsTestMain(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestAlpha"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{
			Clause:       map[string]string{"TestAlpha": "p", "TestMain": "p"},
			TestParallel: map[string]bool{"TestAlpha": true},
		},
	}
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if s.RunRegex != "" {
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
		}
		if s.Parallel >= 2 {
			return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusFail}), nil
		}
		return gotest.Result(s, map[string]domain.Status{"TestAlpha": domain.StatusPass}), nil
	}}

	if _, err := (probe.Parallel{}).Probe(context.Background(), tgt, fake); !errors.Is(err, domain.ErrNotReproduced) {
		t.Fatalf("TestMain is the only candidate: no witness should be built, got %v", err)
	}
}

// Without t.Parallel() the class is impossible: the registry must not call the
// probe at all and must report it as "impossible" — not as "tried, does not explain".
func TestParallelGateRunsNothingAndSaysWhy(t *testing.T) {
	tgt := probe.Target{
		Test:  domain.TestID{Package: "p", Name: "TestAlpha"},
		Dir:   t.TempDir(),
		Facts: facts.PkgFacts{Clause: map[string]string{"TestAlpha": "p", "TestBeta": "p"}},
	}
	fake := &gotest.Fake{}
	res, err := probe.NewRegistry(probe.Parallel{}).Run(context.Background(), tgt, fake)
	if !errors.Is(err, probe.ErrNotReproduced) {
		t.Fatalf("expected ErrNotReproduced, got %v", err)
	}
	if fake.CallCount() != 0 {
		t.Errorf("the static gate spent runs: %d", fake.CallCount())
	}
	if len(res.Tried) != 1 || !res.Tried[0].Skipped {
		t.Fatalf("the skip must be reported: %+v", res.Tried)
	}
	if !strings.Contains(res.Tried[0].Reason, "t.Parallel") {
		t.Errorf("the reason must state the reason: %q", res.Tried[0].Reason)
	}
}

// A package whose only overlap partner lives in the other clause: the shape is
// real (the tests do share one binary and one -parallel pool), but the witness
// file is generated into the victim's clause and cannot call the other clause's
// functions. The answer must be ErrUnsupportedTest — never ErrInternal, which is
// what "witness did not compile" used to produce, and which ddmin propagated
// straight out of the probe.
func TestParallelReportsCrossClauseInsteadOfInternal(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestOuterVictim"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{
			Clause:       map[string]string{"TestOuterVictim": "p_test", "TestInner": "p"},
			TestParallel: map[string]bool{"TestOuterVictim": true, "TestInner": true},
		},
	}

	witnessRuns := 0
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
			// No witness may be attempted: a cross-clause name in it is a
			// compile error, and attempting it is the bug under test.
			witnessRuns++
			res := gotest.Result(s, nil)
			res.BuildFailed = true
			return res, nil
		case s.RunRegex != "":
			return gotest.Result(s, map[string]domain.Status{"TestOuterVictim": domain.StatusPass}), nil
		case s.Parallel >= 2:
			return gotest.Result(s, map[string]domain.Status{
				"TestOuterVictim": domain.StatusFail, "TestInner": domain.StatusPass,
			}), nil
		default:
			return gotest.Result(s, map[string]domain.Status{
				"TestOuterVictim": domain.StatusPass, "TestInner": domain.StatusPass,
			}), nil
		}
	}}

	_, err := probe.Parallel{}.Probe(context.Background(), tgt, fake)
	if err == nil {
		t.Fatal("a cross-clause package must be refused, not silently reported clean")
	}
	if !errors.Is(err, domain.ErrUnsupportedTest) {
		t.Fatalf("expected ErrUnsupportedTest, got %v", err)
	}
	if errors.Is(err, domain.ErrInternal) {
		t.Fatalf("ErrInternal is the bug: %v", err)
	}
	if witnessRuns != 0 {
		t.Errorf("no witness may be built with a cross-clause candidate, got %d build(s)", witnessRuns)
	}
	// The message must name both tests and both clauses: "unsupported" alone
	// leaves the reader guessing what defeated the tool.
	for _, want := range []string{"TestInner", "TestOuterVictim", "package p", "p_test"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q: %v", want, err)
		}
	}
}

// Same-clause candidates exist but none reproduces the overlap, and the package
// does contain a test from the other clause: the missing polluter is across the
// clause boundary. Reporting "does not explain" would read as "no flake".
func TestParallelReportsCrossClauseWhenSameClauseFailsToReproduce(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestOuterVictim"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{
			Clause: map[string]string{
				"TestOuterVictim": "p_test",
				"TestSibling":     "p_test",
				"TestInner":       "p",
			},
			TestParallel: map[string]bool{
				"TestOuterVictim": true, "TestSibling": true, "TestInner": true,
			},
		},
	}

	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
			// The victim passes inside the witness: the same-clause sibling
			// does not explain the overlap.
			return gotest.Result(s, nil), nil
		case s.RunRegex != "":
			return gotest.Result(s, map[string]domain.Status{"TestOuterVictim": domain.StatusPass}), nil
		case s.Parallel >= 2:
			return gotest.Result(s, map[string]domain.Status{
				"TestOuterVictim": domain.StatusFail,
				"TestSibling":     domain.StatusPass,
				"TestInner":       domain.StatusPass,
			}), nil
		default:
			return gotest.Result(s, map[string]domain.Status{
				"TestOuterVictim": domain.StatusPass,
				"TestSibling":     domain.StatusPass,
				"TestInner":       domain.StatusPass,
			}), nil
		}
	}}

	_, err := probe.Parallel{}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrUnsupportedTest) {
		t.Fatalf("expected ErrUnsupportedTest, got %v", err)
	}
}

// The filter must not over-refuse: with a same-clause polluter present, the
// witness is built from it alone and the cross-clause test is never called.
func TestParallelIgnoresCrossClauseWhenSameClauseExplains(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestAlpha"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{
			Clause: map[string]string{
				"TestAlpha": "p", "TestBeta": "p", "TestInner": "p_test",
			},
			TestParallel: map[string]bool{"TestAlpha": true, "TestBeta": true, "TestInner": true},
		},
	}

	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
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
				"TestAlpha": domain.StatusFail, "TestBeta": domain.StatusPass, "TestInner": domain.StatusPass,
			}), nil
		default:
			return gotest.Result(s, map[string]domain.Status{
				"TestAlpha": domain.StatusPass, "TestBeta": domain.StatusPass, "TestInner": domain.StatusPass,
			}), nil
		}
	}}

	w, err := probe.Parallel{}.Probe(context.Background(), tgt, fake)
	if err != nil {
		t.Fatalf("a same-clause witness was expected: %v", err)
	}
	if len(w.Polluters) != 1 || w.Polluters[0].Name != "TestBeta" {
		t.Fatalf("polluters = %+v, want only TestBeta (TestInner is not callable)", w.Polluters)
	}
	for _, st := range w.Steps {
		if st.Test.Name == "TestInner" {
			t.Fatalf("cross-clause test reached the witness steps: %+v", st)
		}
	}
}
