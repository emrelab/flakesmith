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

func envTarget(dir string) probe.Target {
	return probe.Target{
		Test: domain.TestID{Package: "p", Name: "TestNeedsEnv"},
		Dir:  dir,
		Facts: facts.PkgFacts{
			Clause:    map[string]string{"TestNeedsEnv": "p"},
			EnvReads:  []string{"FIXTURE_MODE"},
			EnvValues: map[string][]string{"FIXTURE_MODE": {"ci"}},
		},
	}
}

// The code is unchanged, only the environment changes: if that is the only difference, the flake is env class.
// Only the key is written to the capsule — the value may be a secret.
func TestEnvFindsKeyWithoutStoringValue(t *testing.T) {
	tgt := envTarget(t.TempDir())
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if hasEnv(s, "FIXTURE_MODE=ci") {
			return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusPass}), nil
		}
		return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusFail}), nil
	}}

	w, err := probe.Env{}.Probe(context.Background(), tgt, fake)
	if err != nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if w.Kind != domain.KindEnv {
		t.Errorf("Kind = %s", w.Kind)
	}
	if len(w.EnvDiff) != 1 || w.EnvDiff[0] != "FIXTURE_MODE" {
		t.Fatalf("EnvDiff = %v", w.EnvDiff)
	}
	for _, v := range w.EnvDiff {
		if strings.Contains(v, "=") {
			t.Errorf("value leaked: %v", w.EnvDiff)
		}
	}
}

// With no candidate values the probe must not guess: trying a value the user
// never wrote in their own code produces an unverifiable finding.
func TestEnvWithoutCandidateValuesSaysNothing(t *testing.T) {
	tgt := envTarget(t.TempDir())
	tgt.Facts.EnvValues = nil
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusFail}), nil
	}}

	_, err := probe.Env{}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrNotReproduced) {
		t.Fatalf("expected ErrNotReproduced, got %v", err)
	}
	if fake.CallCount() > 1 {
		t.Errorf("wasted run: %d calls", fake.CallCount())
	}
}

// If the difference appears once and vanishes it is not deterministic: writing
// "always" into the capsule would be a lie.
func TestEnvRequiresDeterminism(t *testing.T) {
	tgt := envTarget(t.TempDir())
	flip := 0
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if hasEnv(s, "FIXTURE_MODE=ci") {
			flip++
			if flip > 1 {
				// No longer passing during verification.
				return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusFail}), nil
			}
			return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusPass}), nil
		}
		return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusFail}), nil
	}}

	_, err := probe.Env{}.Probe(context.Background(), tgt, fake)
	if !errors.Is(err, domain.ErrNotReproducible) {
		t.Fatalf("expected ErrNotReproducible, got %v", err)
	}
}

func hasEnv(s domain.RunSpec, want string) bool {
	for _, e := range s.Env {
		if e == want {
			return true
		}
	}
	return false
}
