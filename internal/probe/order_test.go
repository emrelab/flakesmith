package probe_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/facts"
	"github.com/emrelab/flakesmith/internal/gotest"
	"github.com/emrelab/flakesmith/internal/probe"
)

// A witness may only call tests from the victim's own test binary. When every
// sibling that ran before the victim belongs to the other clause there is no
// witness to build at all — the probe must say so instead of reporting
// "no flake", which would claim knowledge it does not have.
func TestOrderReportsCrossClauseInsteadOfNoFlake(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestVictim"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{Clause: map[string]string{
			"TestVictim":        "p_test",
			"TestInnerPolluter": "p",
		}},
	}

	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if s.RunRegex == "^TestVictim$" {
			return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusPass}), nil
		}
		// Whole package: the inner-package test runs first, the victim fails.
		res := gotest.Result(s, map[string]domain.Status{
			"TestInnerPolluter": domain.StatusPass,
			"TestVictim":        domain.StatusFail,
		})
		res.Order = []string{"TestInnerPolluter", "TestVictim"}
		return res, nil
	}}

	_, err := probe.Order{Seeds: []int64{1}}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrUnsupportedTest) {
		t.Fatalf("expected ErrUnsupportedTest, got %v", err)
	}
	// The message must name both clauses: "unsupported" alone does not tell
	// the user which test in which package defeated the tool.
	for _, want := range []string{"TestInnerPolluter", "package p", "p_test"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q: %v", want, err)
		}
	}
}

// Same-clause candidates exist but none reproduces the failure, and the
// package does contain tests from the other clause: the missing ingredient is
// across the binary boundary. That is unsupported, not "clean".
func TestOrderReportsCrossClauseWhenSameClauseFailsToReproduce(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestVictim"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{Clause: map[string]string{
			"TestVictim":        "p_test",
			"TestSibling":       "p_test",
			"TestInnerPolluter": "p",
		}},
	}

	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case s.RunRegex == "^TestVictim$":
			return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusPass}), nil
		case strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
			// The victim inside the witness passes: same-clause candidates do not explain it.
			return gotest.Result(s, nil), nil
		default:
			res := gotest.Result(s, map[string]domain.Status{
				"TestSibling":       domain.StatusPass,
				"TestInnerPolluter": domain.StatusPass,
				"TestVictim":        domain.StatusFail,
			})
			res.Order = []string{"TestSibling", "TestInnerPolluter", "TestVictim"}
			return res, nil
		}
	}}

	_, err := probe.Order{Seeds: []int64{1}}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrUnsupportedTest) {
		t.Fatalf("expected ErrUnsupportedTest, got %v", err)
	}
}

// Single-clause packages must behave unchanged: the same-clause polluter is found.
func TestOrderSingleClauseStillFindsPolluter(t *testing.T) {
	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestVictim"},
		Dir:  t.TempDir(),
		Facts: facts.PkgFacts{Clause: map[string]string{
			"TestVictim":   "p",
			"TestPolluter": "p",
		}},
	}

	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case s.RunRegex == "^TestVictim$":
			return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusPass}), nil
		case strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
			// The witness brings the victim down.
			name := strings.Trim(s.RunRegex, "^$")
			res := gotest.Result(s, nil)
			res.Outcomes[domain.WitnessSubtest(name, "TestVictim")] = domain.Outcome{Status: domain.StatusFail}
			return res, nil
		default:
			res := gotest.Result(s, map[string]domain.Status{
				"TestPolluter": domain.StatusPass,
				"TestVictim":   domain.StatusFail,
			})
			res.Order = []string{"TestPolluter", "TestVictim"}
			return res, nil
		}
	}}

	w, err := probe.Order{Seeds: []int64{1}}.Probe(context.Background(), tgt, fake)
	if err != nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if len(w.Polluters) != 1 || w.Polluters[0].Name != "TestPolluter" {
		t.Fatalf("polluter = %+v", w.Polluters)
	}
}

// Ctrl-C mid-search must leave no witness file: leftover debris breaks every
// later build in the package and the user has to delete it by hand.
func TestOrderCancelLeavesNoWitnessFile(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tgt := probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestVictim"},
		Dir:  dir,
		Facts: facts.PkgFacts{Clause: map[string]string{
			"TestVictim":   "p",
			"TestPolluter": "p",
		}},
	}

	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		switch {
		case s.RunRegex == "^TestVictim$":
			return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusPass}), nil
		case strings.HasPrefix(s.RunRegex, "^TestFlakeWitness"):
			cancel() // the user pressed Ctrl-C
			return nil, context.Canceled
		default:
			res := gotest.Result(s, map[string]domain.Status{
				"TestPolluter": domain.StatusPass,
				"TestVictim":   domain.StatusFail,
			})
			res.Order = []string{"TestPolluter", "TestVictim"}
			return res, nil
		}
	}}

	_, err := probe.Order{Seeds: []int64{1}}.Probe(ctx, tgt, fake)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected a cancellation error, got %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, "zz_flakesmith_witness_*"))
	if len(leftovers) > 0 {
		t.Errorf("witness files left behind: %v", leftovers)
	}
}
