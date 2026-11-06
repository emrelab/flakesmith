package gotest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emrelab/flakesmith/internal/domain"
)

// fixture resolves testdata/fixtures/<name> from this package's directory.
func fixture(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture missing: %s", p)
	}
	return p
}

func newExec(t *testing.T) *Exec {
	t.Helper()
	if _, err := os.Stat("/usr/bin/env"); err != nil {
		t.Skip("exec unavailable")
	}
	return &Exec{}
}

// Discover must preserve source order: this is the order probe's baseline.
func TestDiscoverSourceOrder(t *testing.T) {
	e := newExec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	got, err := e.Discover(ctx, fixture(t, "order-basic"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"TestPolluter", "TestVictim", "TestUnaffected"} // order in state_test.go + checkout_test.go
	if len(got) != len(want) {
		t.Fatalf("Discover = %v, want %v", names(got), want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("[%d] = %q, want %q (source order must be preserved)", i, got[i].Name, want[i])
		}
	}
	if got[0].Package == "" || got[0].Package == "." {
		t.Errorf("Package must be the import path, got %q", got[0].Package)
	}
}

// A failing test is DATA, not an error.
func TestRunFailingTestIsData(t *testing.T) {
	e := newExec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	res, err := e.Run(ctx, domain.RunSpec{
		Package:  fixture(t, "order-basic"),
		RunRegex: "^(TestPolluter|TestVictim)$",
		Count:    1,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		t.Fatalf("a failing test must not return an error: %v", err)
	}
	if !res.Failed("TestVictim") {
		t.Errorf("TestVictim should have failed, Outcomes=%v", res.Outcomes)
	}
	if !res.Passed("TestPolluter") {
		t.Error("TestPolluter should have passed")
	}
	if res.ExitCode == 0 {
		t.Error("a failing test must not have exit code 0")
	}
	if res.BuildFailed {
		t.Error("this is not a build failure")
	}
	if len(res.Order) != 2 {
		t.Errorf("Order = %v", res.Order)
	}
}

// The victim running alone must pass: the proof of order dependence.
func TestRunVictimAlonePasses(t *testing.T) {
	e := newExec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	res, err := e.Run(ctx, domain.RunSpec{
		Package:  fixture(t, "order-basic"),
		RunRegex: "^TestVictim$",
		Count:    1,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed("TestVictim") {
		t.Fatalf("should have passed alone: %v", res.Outcomes)
	}
}

// -count=2 catches the state leak; the worst status must be fail.
func TestRunCountDetectsAccumulation(t *testing.T) {
	e := newExec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	dir := fixture(t, "repeat-accumulate")
	spec := domain.RunSpec{Package: dir, RunRegex: "^TestAccumulates$", Count: 2, Timeout: 60 * time.Second}

	res, err := e.Run(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed("TestAccumulates") {
		t.Fatalf("-count=2 should have failed: %v", res.Outcomes)
	}
}

// A compile error must set BuildFailed, NOT wrap ErrBuildFailed (this table
// becomes a decision in the calling layer).
func TestRunBuildFailure(t *testing.T) {
	e := newExec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module broken\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "broken_test.go"), "package broken\n\nfunc TestBroken(t *testing.T) { thisIsASyntaxError }\n")

	res, err := e.Run(ctx, domain.RunSpec{Package: dir, Count: 1, Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("a build failure must be returned as data: %v", err)
	}
	if !res.BuildFailed {
		t.Fatalf("expected BuildFailed=true, Outcomes=%v", res.Outcomes)
	}
	if res.BuildOutput == "" {
		t.Error("BuildOutput must be non-empty (the user should see what is broken)")
	}
}

// Fake backs the upper-layer tests: records specs, returns a scripted universe.
func TestFakeRecordsSpecs(t *testing.T) {
	f := &Fake{RunFn: func(s domain.RunSpec) (*domain.RunResult, error) {
		return Result(s, map[string]domain.Status{"TestA": domain.StatusFail}), nil
	}}
	if _, err := f.Run(context.Background(), domain.RunSpec{Count: 1}); err != nil {
		t.Fatal(err)
	}
	if f.CallCount() != 1 {
		t.Fatalf("CallCount = %d", f.CallCount())
	}
	if f.Runs()[0].Count != 1 {
		t.Error("the spec must be recorded")
	}
}

func names(ids []domain.TestID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Name
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestExecShortFlag pins the operator-level -short plumbing with a fake go
// binary that records its arguments: --short (Exec.Short) must reach the
// go command on every invocation, and must not when the flag is off. The
// runner is the only place that builds go arguments, so this single test
// covers every hunt/repro path.
func TestExecShortFlag(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	shim := filepath.Join(dir, "go")
	script := "#!/bin/sh\necho \"$@\" > " + shellQuote(argsFile) +
		"\nprintf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"shim\"}'\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		short bool
		want  bool
	}{{true, true}, {false, false}} {
		if err := os.Remove(argsFile); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		e := &Exec{GoBin: shim, Short: tc.short}
		if _, err := e.Run(context.Background(), domain.RunSpec{Package: dir, Timeout: 30 * time.Second}); err != nil {
			t.Fatalf("Short=%v: Run: %v", tc.short, err)
		}
		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("Short=%v: args not recorded: %v", tc.short, err)
		}
		has := strings.Contains(string(data), "-short")
		if has != tc.want {
			t.Errorf("Short=%v: args contain -short=%v, want %v (%q)", tc.short, has, tc.want, data)
		}
	}
}

// shellQuote keeps the shim script independent of tempdir path quirks.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
