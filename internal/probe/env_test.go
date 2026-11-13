package probe_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/facts"
	"github.com/emrelab/go-flake-hunter/internal/gotest"
	"github.com/emrelab/go-flake-hunter/internal/probe"
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

// #2A: a variable the test never reads itself — the package's application code
// reads it — is still a candidate: the witness runs the real `go test`, so an
// indirect read flips the outcome just the same. But app code is noisier, so
// when BOTH sides flip, the test-local key must be the finding.
func TestEnvTriesAppKeysAfterTestKeys(t *testing.T) {
	tgt := envTarget(t.TempDir())
	tgt.Facts.AppEnvReads = []string{"DEPLOY_TARGET"}
	tgt.Facts.EnvValues["DEPLOY_TARGET"] = []string{"staging"}

	// Both values flip the test (baseline fails, either variable set passes).
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if hasEnv(s, "FIXTURE_MODE=ci") || hasEnv(s, "DEPLOY_TARGET=staging") {
			return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusPass}), nil
		}
		return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusFail}), nil
	}}

	w, err := probe.Env{}.Probe(context.Background(), tgt, fake)
	if err != nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if len(w.EnvDiff) != 1 || w.EnvDiff[0] != "FIXTURE_MODE" {
		t.Errorf("test-local key must win the candidate race, EnvDiff = %v", w.EnvDiff)
	}
}

// An app-only key (no literal read in any _test.go file) must still make the
// probe applicable and produce a witness: this is exactly the env flake the
// test-file-only scan used to miss.
func TestEnvWitnessFromAppOnlyKey(t *testing.T) {
	tgt := envTarget(t.TempDir())
	tgt.Facts.EnvReads = nil
	tgt.Facts.AppEnvReads = []string{"DEPLOY_TARGET"}
	tgt.Facts.EnvValues = map[string][]string{"DEPLOY_TARGET": {"staging"}}

	if !(probe.Env{}).Applicable(tgt) {
		t.Fatal("an app-file env key must make the env probe applicable")
	}
	fake := &gotest.Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		if hasEnv(s, "DEPLOY_TARGET=staging") {
			return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusFail}), nil
		}
		return gotest.Result(s, map[string]domain.Status{"TestNeedsEnv": domain.StatusPass}), nil
	}}

	w, err := probe.Env{}.Probe(context.Background(), tgt, fake)
	if err != nil {
		t.Fatalf("a witness was expected: %v", err)
	}
	if len(w.EnvDiff) != 1 || w.EnvDiff[0] != "DEPLOY_TARGET" {
		t.Errorf("EnvDiff = %v, want [DEPLOY_TARGET]", w.EnvDiff)
	}
}
