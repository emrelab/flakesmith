package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/emrelab/flakesmith/internal/domain"
)

func TestRunSpecWithDefaults(t *testing.T) {
	tests := []struct {
		name      string
		in        domain.RunSpec
		wantCount int
		wantTO    time.Duration
	}{
		{"zero count breaks cache", domain.RunSpec{Package: "p"}, 1, domain.DefaultTimeout},
		{"negative count is fixed", domain.RunSpec{Count: -3}, 1, domain.DefaultTimeout},
		{"existing values are kept", domain.RunSpec{Count: 10, Timeout: 5 * time.Second}, 10, 5 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.WithDefaults()
			if got.Count != tc.wantCount {
				t.Errorf("Count = %d, want %d", got.Count, tc.wantCount)
			}
			if got.Timeout != tc.wantTO {
				t.Errorf("Timeout = %s, want %s", got.Timeout, tc.wantTO)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{nil, domain.ExitOK},
		{domain.ErrFlakeFound, domain.ExitFlakeFound},
		{domain.ErrUsage, domain.ExitUsage},
		{domain.ErrNotReproduced, domain.ExitNotReproduced},
		{domain.ErrNotReproducible, domain.ExitNotReproduced},
		{domain.ErrBudgetExceeded, domain.ExitBudget},
		{domain.ErrUnsupportedTest, domain.ExitUnsupported},
		{domain.ErrBuildFailed, domain.ExitBuildFailed},
		// Cancellation is not a fault but the user's decision: 10 (internal error) would be wrong.
		{context.Canceled, domain.ExitCanceled},
		{context.DeadlineExceeded, domain.ExitCanceled},
		{errors.New("unknown"), domain.ExitInternal},
	}
	for _, tc := range tests {
		if got := domain.ExitCode(tc.err); got != tc.want {
			t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

// A wrapped error must keep its identity: the exit code must not slip while the CLI adds a message.
func TestExitCodeSurvivesWrap(t *testing.T) {
	wrapped := domain.Wrap(domain.ErrBudgetExceeded, "hunt %s", "./...")
	if got := domain.ExitCode(wrapped); got != domain.ExitBudget {
		t.Fatalf("ExitCode(wrapped) = %d, want %d", got, domain.ExitBudget)
	}
	if !errors.Is(wrapped, domain.ErrBudgetExceeded) {
		t.Fatal("errors.Is does not work after wrapping")
	}
}

func TestNames(t *testing.T) {
	// Files prefixed with "_" or "." are not compiled; the temporary harness must start with "zz_".
	got := domain.WitnessFileName("0")
	if got != "zz_flakesmith_witness_0_test.go" {
		t.Errorf("WitnessFileName = %q", got)
	}
	if !domain.IsWitnessFileName(got) {
		t.Error("IsWitnessFileName did not recognize the name it produced")
	}
	if domain.IsWitnessFileName("checkout_test.go") {
		t.Error("a user file was mistaken for a harness")
	}
	if p := domain.PersistentFileName("checkout_test.go"); p != "checkout_flakesmith_test.go" {
		t.Errorf("PersistentFileName = %q", p)
	}
	if p := domain.PersistentFileName("order.go"); p != "order_flakesmith_test.go" {
		t.Errorf("PersistentFileName(.go) = %q", p)
	}
	if f := domain.PersistentFuncName("TestCheckout"); f != "TestFlakeRegression_TestCheckout" {
		t.Errorf("PersistentFuncName = %q", f)
	}
}

func TestKindValid(t *testing.T) {
	for _, k := range []domain.Kind{domain.KindRepeat, domain.KindOrder, domain.KindParallel, domain.KindEnv} {
		if !k.Valid() {
			t.Errorf("%q should have been valid", k)
		}
	}
	if domain.Kind("fake").Valid() {
		t.Error("an unknown class was accepted as valid")
	}
}

func TestRunResultStatus(t *testing.T) {
	r := &domain.RunResult{Outcomes: map[string]domain.Outcome{
		"TestA": {Status: domain.StatusPass},
		"TestB": {Status: domain.StatusFail},
	}}
	if !r.Passed("TestA") || r.Failed("TestA") {
		t.Error("TestA should pass")
	}
	if !r.Failed("TestB") {
		t.Error("TestB should fail")
	}
	if r.Status("TestYok") != domain.StatusSkip {
		t.Error("an unknown test should return skip")
	}
	var nilRes *domain.RunResult
	if nilRes.Status("TestA") != domain.StatusSkip {
		t.Error("a nil result must not panic")
	}
}

func TestWitnessMinimal(t *testing.T) {
	if (&domain.Witness{Runs: 5}).Minimal() {
		t.Error("no polluter means not minimal")
	}
	w := domain.Witness{Polluters: []domain.TestID{{Name: "TestP"}}, Runs: 2}
	if !w.Minimal() {
		t.Error("polluter + 2 runs should be minimal")
	}
	o := domain.Observe(3, 2)
	w.Verification = &o
	if w.Minimal() {
		t.Error("suspected witness cannot claim verified minimality")
	}
}
