package clireplay

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/cli"
	"github.com/emrelab/flakesmith/internal/clitest"
	"github.com/emrelab/flakesmith/internal/domain"
)

// replay: re-runs the witness. For env flakes the value is not in the capsule,
// so it must honestly refuse without --env.
func TestReplayRefusesEnvCapsuleWithoutValue(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := clitest.FixtureDir(t, "env-dependent")
	store := filepath.Join(t.TempDir(), "c")
	if code, _, errOut := clitest.Run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	code, _, errOut := clitest.Run("replay", "-capsule-dir", store)
	if code != domain.ExitUsage {
		t.Fatalf("code = %d, want %d (%s)", code, domain.ExitUsage, errOut)
	}
	if !strings.Contains(errOut, "--env") {
		t.Errorf("should name the fix: %q", errOut)
	}
}

// Replay path per class: repeat runs with -count, env with --env, order and
// parallel with the witness chain. If all three collapse to one code, the
// "deterministic" report is unevidenced.
func TestReplayKindPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	cases := []struct {
		fixture string
		env     string
	}{
		{"order-basic", ""},
		{"repeat-accumulate", ""},
		{"env-dependent", "FIXTURE_MODE=ci"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			dir := clitest.FixtureDir(t, tc.fixture)
			store := filepath.Join(t.TempDir(), "c")
			if code, _, errOut := clitest.Run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
				t.Fatalf("hunt code=%d %s", code, errOut)
			}
			args := []string{"replay", "-capsule-dir", store, "-n", "2"}
			if tc.env != "" {
				args = append(args, "--env", tc.env)
			}
			code, out, errOut := clitest.Run(args...)
			if code != domain.ExitOK {
				t.Fatalf("replay code=%d %s\n%s", code, errOut, out)
			}
			if !strings.Contains(out, "2/2 reproduced") || !strings.Contains(out, "deterministic") {
				t.Errorf("expected a deterministic report:\n%s", out)
			}
		})
	}
}

// Unusable capsules must be refused without running: a half-written capsule
// has no verified witness, broken JSON has no fields at all, and -n 0 asks no
// question. In all three cases the only right answer is a usage error.
func TestReplayRejectsUnusableCapsule(t *testing.T) {
	store := t.TempDir()
	confirmed := func(id string) domain.Capsule {
		return domain.Capsule{
			Schema: domain.SchemaVersion,
			ID:     id,
			Kind:   domain.KindRepeat,
			Witness: domain.Witness{
				Kind:   domain.KindRepeat,
				Victim: domain.TestID{Name: "TestAccumulates", Package: "example.com/m"},
				Runs:   2,
			},
			Target: domain.Target{Package: "example.com/m", Victim: "TestAccumulates"},
			Status: domain.StatusConfirmed,
		}
	}
	partial := confirmed("fs_partial")
	partial.Status = domain.StatusPartial
	partial.Note = "budget exceeded: 1/1 runs"
	if _, err := capsule.NewStore(store).Write(partial); err != nil {
		t.Fatalf("could not write capsule: %v", err)
	}
	if _, err := capsule.NewStore(store).Write(confirmed("fs_ok")); err != nil {
		t.Fatalf("could not write capsule: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store, "fs_broken.json"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"two arguments", []string{"replay", "a", "b"}, "at most one capsule"},
		{"partial capsule", []string{"replay", "fs_partial", "-capsule-dir", store}, "partial capsule"},
		{"broken json", []string{"replay", "fs_broken", "-capsule-dir", store}, "capsule is corrupt"},
		{"zero runs", []string{"replay", "fs_ok", "-capsule-dir", store, "-n", "0"}, "-n must be positive"},
		{"negative runs", []string{"replay", "fs_ok", "-capsule-dir", store, "-n", "-3"}, "-n must be positive"},
		{"missing capsule", []string{"replay", "fs_yok", "-capsule-dir", store}, "capsule not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := clitest.Run(tc.args...)
			if code != domain.ExitUsage {
				t.Fatalf("code = %d, want %d (%s)", code, domain.ExitUsage, errOut)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("stderr %q should contain %q", errOut, tc.want)
			}
		})
	}
}

// repro targets one test: no need for hunt to scan the whole package, the
// budget goes to the single suspect.
func TestReproTargetsOneTest(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := clitest.FixtureDir(t, "order-basic")
	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"repro", dir, "TestVictim", "-capsule-dir", filepath.Join(t.TempDir(), "c")},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitOK {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "polluter     : TestPolluter") {
		t.Errorf("witness not narrowed:\n%s", out.String())
	}
}

// repro answers a single question: no answer means exit 3. That is the difference from hunt.
func TestReproNotReproducedExitsThree(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := clitest.FixtureDir(t, "clean")
	code, out, _ := clitest.Run("repro", dir, "TestSum", "-capsule-dir", t.TempDir())
	if code != domain.ExitNotReproduced {
		t.Fatalf("code = %d, want %d\n%s", code, domain.ExitNotReproduced, out)
	}
}

// repro --from-json (#5): the CI log already paid for the discovery; repro owes
// only the witness. The env-dependent fixture is the target of a hand-written
// failure log, and the import path in that log must resolve to the fixture
// through `go list` — which is why the command must run inside the repo the
// log came from (t.Chdir here is the CI checkout).
func TestReproFromJSONLog(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := clitest.FixtureDir(t, "env-dependent")
	t.Chdir(dir)

	log := `{"Action":"start","Package":"fixture/envdependent"}
{"Action":"run","Package":"fixture/envdependent","Test":"TestDependsOnEnv"}
{"Action":"fail","Package":"fixture/envdependent","Test":"TestDependsOnEnv"}
`
	logFile := filepath.Join(t.TempDir(), "failure.json")
	if err := os.WriteFile(logFile, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "c")

	code, out, errOut := clitest.Run("repro", "--from-json", logFile,
		"-capsule-dir", store, "--timeout", "150s", "--budget", "400")
	if code != domain.ExitOK {
		t.Fatalf("code=%d\n%s\n%s", code, out, errOut)
	}
	ids, err := capsule.NewStore(store).List()
	if err != nil || len(ids) == 0 {
		t.Fatalf("no capsule written: %v", err)
	}
	c, err := capsule.NewStore(store).Read(ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != domain.KindEnv || c.Target.Victim != "TestDependsOnEnv" {
		t.Errorf("capsule = %s/%s, want env/TestDependsOnEnv", c.Kind, c.Target.Victim)
	}
}

// A failing log with a package this repo does not have is a warning, not a
// veto; a log with no failures says so honestly with exit 3.
func TestReproFromJSONEdgeCases(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		f := filepath.Join(t.TempDir(), "failure.json")
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return f
	}

	f := write(t, `{"Action":"pass","Package":"p","Test":"TestGreen"}`+"\n")
	if code, _, errOut := clitest.Run("repro", "--from-json", f); code != domain.ExitNotReproduced {
		t.Errorf("clean log: code=%d want %d (%s)", code, domain.ExitNotReproduced, errOut)
	}

	f = write(t, "this is not a test2json log\nat all\n")
	if code, _, _ := clitest.Run("repro", "--from-json", f); code != domain.ExitUsage {
		t.Errorf("garbage log: code=%d want %d", code, domain.ExitUsage)
	}

	// Unknown package: warn and continue; with no other suspects the honest
	// answer is still "not reproduced" (exit 3), not a crash.
	f = write(t, `{"Action":"fail","Package":"example.com/not/here","Test":"TestGone"}`+"\n")
	code, _, errOut := clitest.Run("repro", "--from-json", f)
	if code != domain.ExitNotReproduced {
		t.Errorf("foreign package: code=%d want %d (%s)", code, domain.ExitNotReproduced, errOut)
	}
	if !strings.Contains(errOut, "cannot be resolved") {
		t.Errorf("the unresolvable package must be named: %q", errOut)
	}
}
