package probe_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/gotest"
	"github.com/emrelab/go-flake-hunter/internal/probe"
)

const (
	crossPolluterDir = "/pkg/p"
	crossVictimDir   = "/pkg/v"
)

// crossTarget is the victim of a two-binary pollution hunt.
func crossTarget() probe.Target {
	return probe.Target{
		Test: domain.TestID{Package: "m/v", Name: "TestVictim"},
		Dir:  crossVictimDir,
	}
}

// crossFake scripts the only universe this probe needs: TestBad leaves state
// (a temp file, in the real fixture) that the victim CONSUMES — the victim
// passes while the state is absent, fails once it is dirty, and resets it.
// That mirrors the file we clean up after ourselves, and it is what makes the
// witness re-flip every round instead of once.
func crossFake(dirty *bool) *gotest.Fake {
	return &gotest.Fake{
		DiscoverFn: func(pkgDir string) ([]domain.TestID, error) {
			switch pkgDir {
			case crossPolluterDir:
				return []domain.TestID{
					{Package: "m/p", Name: "TestNoop"},
					{Package: "m/p", Name: "TestBad"},
				}, nil
			default:
				return []domain.TestID{{Package: "m/v", Name: "TestVictim"}}, nil
			}
		},
		RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
			if s.Package == crossVictimDir {
				status := domain.StatusPass
				if *dirty {
					status = domain.StatusFail
					*dirty = false // the victim consumes the pollution
				}
				return gotest.Result(s, map[string]domain.Status{"TestVictim": status}), nil
			}
			if strings.Contains(s.RunRegex, "TestBad") {
				*dirty = true
			}
			return gotest.Result(s, map[string]domain.Status{}), nil
		},
	}
}

// #6 happy path: the victim passes alone, fails only after the polluter
// package ran, and ddmin must drop the innocent TestNoop. The witness is
// flagged cross-binary and carries the polluter's package on every step.
func TestOrderCrossShrinksToDirtyingTest(t *testing.T) {
	dirty := false
	w, err := probe.OrderCross{PolluterDir: crossPolluterDir}.Probe(
		context.Background(), crossTarget(), crossFake(&dirty))
	if err != nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if !w.CrossBinary || w.Kind != domain.KindOrder {
		t.Errorf("witness = %+v", w)
	}
	if len(w.Polluters) != 1 || w.Polluters[0].Name != "TestBad" {
		t.Fatalf("polluters = %v, want [TestBad] (TestNoop must shrink away)", w.Polluters)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("steps = %v", w.Steps)
	}
	if w.Steps[0].Package != "m/p" || w.Steps[1].Package != "m/v" {
		t.Errorf("steps must carry their packages: %+v", w.Steps)
	}
	if w.Runs < 2 {
		t.Errorf("Runs = %d, the witness must be re-verified across processes", w.Runs)
	}
}

// The victim failing ALONE is a broken test, not a polluted one: without the
// passing alone-baseline the witness would claim "after TestBad" on top of a
// test that fails anyway. The probe must spend exactly one run and stop.
func TestOrderCrossRequiresAlonePass(t *testing.T) {
	fake := &gotest.Fake{
		DiscoverFn: func(pkgDir string) ([]domain.TestID, error) {
			return []domain.TestID{{Package: "m/p", Name: "TestBad"}}, nil
		},
		RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
			if s.Package == crossVictimDir {
				return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusFail}), nil
			}
			return gotest.Result(s, map[string]domain.Status{}), nil
		},
	}
	_, err := probe.OrderCross{PolluterDir: crossPolluterDir}.Probe(context.Background(), crossTarget(), fake)
	if !errors.Is(err, domain.ErrNotReproduced) {
		t.Fatalf("err = %v, want ErrNotReproduced", err)
	}
	if fake.CallCount() != 1 {
		t.Errorf("the alone-fails case must stop at the baseline, ran %d times", fake.CallCount())
	}
}

// No pollution across the boundary: the honest answer is "not this class",
// not a shrunk set that never reproduced.
func TestOrderCrossSaysNothingWithoutFlip(t *testing.T) {
	fake := &gotest.Fake{
		DiscoverFn: func(pkgDir string) ([]domain.TestID, error) {
			return []domain.TestID{{Package: "m/p", Name: "TestBad"}}, nil
		},
		RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
			return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusPass}), nil
		},
	}
	_, err := probe.OrderCross{PolluterDir: crossPolluterDir}.Probe(context.Background(), crossTarget(), fake)
	if !errors.Is(err, domain.ErrNotReproduced) {
		t.Fatalf("err = %v, want ErrNotReproduced", err)
	}
}

// refusalProbe and finderProbe script the registry rule of #6: a probe that
// cannot EXPRESS a finding (cross-clause order) must not veto the probe that
// can (OrderCross) — but if nothing reproduces, the refusal is still the
// final answer, never a silent "no flake".
type refusalProbe struct{}

func (refusalProbe) Name() string      { return "refuse" }
func (refusalProbe) Kind() domain.Kind { return domain.KindOrder }
func (refusalProbe) Probe(context.Context, probe.Target, gotest.Runner) (*domain.Witness, error) {
	return nil, domain.Wrap(domain.ErrUnsupportedTest, "not in the same test binary")
}

type finderProbe struct{}

func (finderProbe) Name() string      { return "find" }
func (finderProbe) Kind() domain.Kind { return domain.KindOrder }
func (finderProbe) Probe(context.Context, probe.Target, gotest.Runner) (*domain.Witness, error) {
	return &domain.Witness{Kind: domain.KindOrder, CrossBinary: true}, nil
}

func TestRegistryRefusalDoesNotVetoLaterProbes(t *testing.T) {
	res, err := probe.NewRegistry(refusalProbe{}, finderProbe{}).
		Run(context.Background(), crossTarget(), &gotest.Fake{})
	if err != nil || res.Witness == nil {
		t.Fatalf("the later probe must still answer: res=%+v err=%v", res, err)
	}
	// The refusal stays visible in the attempt list, not lost.
	var sawRefusal bool
	for _, a := range res.Tried {
		if errors.Is(a.Err, domain.ErrUnsupportedTest) {
			sawRefusal = true
		}
	}
	if !sawRefusal {
		t.Error("the refusal must remain in the tried list")
	}
}

func TestRegistryRefusalSurvivesWhenNothingReproduces(t *testing.T) {
	_, err := probe.NewRegistry(refusalProbe{}).
		Run(context.Background(), crossTarget(), &gotest.Fake{})
	if !errors.Is(err, domain.ErrUnsupportedTest) {
		t.Fatalf("err = %v, want the refusal back (exit 5), not \"no flake\"", err)
	}
}

func TestOrderCrossDoesNotRunVictimAfterFailedPolluter(t *testing.T) {
	victims := 0
	fake := &gotest.Fake{DiscoverFn: func(string) ([]domain.TestID, error) { return []domain.TestID{{Name: "TestBad"}}, nil }, RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if s.Package == crossVictimDir {
			victims++
			return gotest.Result(s, map[string]domain.Status{"TestVictim": domain.StatusPass}), nil
		}
		res := gotest.Result(s, map[string]domain.Status{"TestBad": domain.StatusFail})
		res.ExitCode = 1
		return res, nil
	}}
	w, err := probe.OrderCross{PolluterDir: crossPolluterDir}.Probe(context.Background(), crossTarget(), fake)
	if !errors.Is(err, probe.ErrNotReproduced) || w != nil || victims != 1 {
		t.Fatalf("w=%+v err=%v victim executions=%d", w, err, victims)
	}
}
