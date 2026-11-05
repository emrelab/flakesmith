package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/cli"
	"github.com/emrelab/flakesmith/internal/domain"
)

func run(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = cli.Execute(context.Background(), args, cli.IO{Stdout: &o, Stderr: &e})
	return code, o.String(), e.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := run("version")
	if code != domain.ExitOK {
		t.Fatalf("code = %d", code)
	}
	if !strings.HasPrefix(out, "flakesmith ") {
		t.Errorf("out = %q", out)
	}
}

// --debug is a global flag: it must be accepted before and after the
// command, and being stripped it must not raise an unknown-flag error.
func TestDebugFlagIsGlobal(t *testing.T) {
	for _, args := range [][]string{{"--debug", "version"}, {"version", "--debug"}} {
		code, out, errOut := run(args...)
		if code != domain.ExitOK {
			t.Fatalf("%v: code=%d err=%s", args, code, errOut)
		}
		if !strings.HasPrefix(out, "flakesmith ") {
			t.Errorf("%v: out = %q", args, out)
		}
	}
}

func TestNoArgsIsUsageError(t *testing.T) {
	code, _, errOut := run()
	if code != domain.ExitUsage {
		t.Fatalf("code = %d, want %d", code, domain.ExitUsage)
	}
	if !strings.Contains(errOut, "Usage") {
		t.Errorf("usage text missing: %q", errOut)
	}
}

func TestHelpListsPlannedCommands(t *testing.T) {
	code, out, _ := run("help")
	if code != domain.ExitOK {
		t.Fatalf("code = %d", code)
	}
	for _, want := range []string{"hunt", "repro", "compile", "replay", "explain", "init"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from help text", want)
		}
	}
}

// TestSubcommandHelpExitsZero: `hunt --help` is a help request, not a usage
// error. If flag.ErrHelp fell into ExitCode, CI would get a 2 (usage error)
// and be misled into thinking the command is broken.
func TestSubcommandHelpExitsZero(t *testing.T) {
	for _, name := range []string{"hunt", "compile", "repro", "replay", "explain", "init"} {
		code, _, errOut := run(name, "--help")
		if code != domain.ExitOK {
			t.Errorf("%s --help: code = %d, want %d", name, code, domain.ExitOK)
		}
		if strings.Contains(errOut, "help requested") {
			t.Errorf("%s --help: help request printed as an error: %q", name, errOut)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	code, _, errOut := run("nope")
	if code != domain.ExitUsage {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("stderr = %q", errOut)
	}
}

// repro without arguments must print usage: silently returning 0 reads as
// "I hunted, it's clean".
func TestReproWithoutArgsIsUsageError(t *testing.T) {
	code, _, errOut := run("repro")
	if code != domain.ExitUsage {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errOut, "usage") {
		t.Errorf("stderr = %q", errOut)
	}
}

// End-to-end: hunt runs on a real fixture and writes a capsule.
// This is the automated form of T5's acceptance criterion.
func TestHuntEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "repeat-accumulate"))
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "capsules")

	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "-capsule-dir", store},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "TestAccumulates") {
		t.Errorf("no test name in output: %s", out.String())
	}

	entries, err := os.ReadDir(store)
	if err != nil || len(entries) != 1 {
		t.Fatalf("exactly one capsule should be written: %v (%d files)", err, len(entries))
	}

	// Same command a second time: the SAME file, not a new copy.
	var out2 bytes.Buffer
	if code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "-capsule-dir", store},
		cli.IO{Stdout: &out2, Stderr: &bytes.Buffer{}}); code != domain.ExitOK {
		t.Fatalf("second run code = %d", code)
	}
	entries2, _ := os.ReadDir(store)
	if len(entries2) != 1 {
		t.Fatalf("must be idempotent: %d files", len(entries2))
	}
}

// With no flake: exit 0 and an honest explanation: which probe said what.
//
// "I looked and found nothing" is a successful scan; turning CI red makes
// people remove the tool on day one in a clean repo. A single asked-about
// test going unanswered is `repro`'s problem (exit 3).
//
// The `clean` fixture is deliberately outside all four classes; "not found"
// here is a real result, not a probe gap.
func TestHuntCleanExitsZero(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "clean")
	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "-capsule-dir", t.TempDir()},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitOK {
		t.Fatalf("code = %d, want %d (stdout=%s)", code, domain.ExitOK, out.String())
	}
	if !strings.Contains(out.String(), "flake not found") {
		t.Errorf("no explanation: %s", out.String())
	}
	// All four probes must have been tried: hiding coverage makes missing
	// coverage look like "clean".
	for _, p := range []string{"repeat", "order", "parallel", "env"} {
		if !strings.Contains(out.String(), p) {
			t.Errorf("probe %s not reported: %s", p, out.String())
		}
	}
}

// When the budget runs out: exit 4, a partial capsule is written, no witness
// file remains. If "cut short" and "clean" share one code, CI is lying.
func TestHuntBudgetWritesPartialCapsule(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "repeat-accumulate"))
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "capsules")

	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "-capsule-dir", store, "--budget", "1"},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitBudget {
		t.Fatalf("code = %d, want %d (stdout=%s stderr=%s)",
			code, domain.ExitBudget, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "status         : partial") {
		t.Errorf("no partial report:\n%s", out.String())
	}

	entries, err := os.ReadDir(store)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one partial capsule: %v %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(store, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"status": "partial"`) {
		t.Errorf("capsule is not partial:\n%s", data)
	}

	// After a cancel/interruption no witness file may remain in the package.
	leftovers, _ := filepath.Glob(filepath.Join(dir, "zz_flakesmith_witness_*"))
	if len(leftovers) > 0 {
		t.Errorf("witness files left behind: %v", leftovers)
	}
}

// Markdown output must be a single document: prose mixed in breaks the table
// in the PR comment.
func TestHuntMarkdownIsASingleDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "order-basic")
	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "-capsule-dir", filepath.Join(t.TempDir(), "c"), "--format", "markdown"},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitOK {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "### flake: TestVictim") || !strings.Contains(s, "| field | value |") {
		t.Fatalf("markdown has no table:\n%s", s)
	}
	if strings.Contains(s, "flake not found") {
		t.Errorf("prose mixed into markdown output:\n%s", s)
	}
}

// --fail-on-flake: the CI gate. A found flake must not pass with code 0.
func TestHuntFailOnFlake(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "repeat-accumulate")
	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "-capsule-dir", filepath.Join(t.TempDir(), "c"), "--fail-on-flake"},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitFlakeFound {
		t.Fatalf("code = %d, want %d", code, domain.ExitFlakeFound)
	}
}

// --race: witness runs happen with -race and the flag is written to the capsule.
//
// Defining the flag in the CLI but wiring it to nothing lets the user say "I
// scanned in race mode"; unless the evidence itself changed, that is a lie.
func TestHuntRaceFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "order-basic")
	store := filepath.Join(t.TempDir(), "c")
	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "--race", "-capsule-dir", store},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitOK {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "go test -race") {
		t.Errorf("-race missing from reproduce command:\n%s", out.String())
	}

	entries, err := os.ReadDir(store)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one capsule: %v %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(store, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"race": true`) {
		t.Errorf("capsule does not carry the race flag:\n%s", data)
	}
}

// fixtureDir copies a fixture module into a temp dir.
//
// Tests must never run inside testdata: hunt writes capsules and temporary
// witness files there, and a second package's test (harness/compile) installs
// its own witness in the same directory. A shared live directory is how one
// test's cleanup deletes another test's witness mid-run — the flake the tool
// is supposed to find, produced by its own test suite.
func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		// flakesmith's own output is not copied: a regression test left over
		// from a manual run would redeclare the same function in the copy.
		for _, marker := range []string{domain.CapsuleDir, domain.PersistentFileSuffix, domain.WitnessFilePrefix} {
			if strings.Contains(path, marker) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("could not copy fixture: %v", err)
	}
	return dst
}

// repro targets one test: no need for hunt to scan the whole package, the
// budget goes to the single suspect.
func TestReproTargetsOneTest(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "order-basic")
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

// explain runs no tests: the report rendered from a capsule must be identical
// even on a machine without `go`.
func TestExplainRerendersCapsule(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "repeat-accumulate")
	store := filepath.Join(t.TempDir(), "c")
	if code, _, errOut := run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	code, out, errOut := run("explain", "-capsule-dir", store, "--format", "markdown")
	if code != domain.ExitOK {
		t.Fatalf("explain code=%d %s", code, errOut)
	}
	if !strings.Contains(out, "| class | repeat |") {
		t.Errorf("explain output missing:\n%s", out)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, "zz_flakesmith_witness_*"))
	if len(leftovers) > 0 {
		t.Errorf("explain must not run anything, left witness files: %v", leftovers)
	}
}

// replay: re-runs the witness. For env flakes the value is not in the capsule,
// so it must honestly refuse without --env.
func TestReplayRefusesEnvCapsuleWithoutValue(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "env-dependent")
	store := filepath.Join(t.TempDir(), "c")
	if code, _, errOut := run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	code, _, errOut := run("replay", "-capsule-dir", store)
	if code != domain.ExitUsage {
		t.Fatalf("code = %d, want %d (%s)", code, domain.ExitUsage, errOut)
	}
	if !strings.Contains(errOut, "--env") {
		t.Errorf("should name the fix: %q", errOut)
	}
}

// A capsule reference must be accepted in three forms: raw id, `ls .flakesmith`
// output (bare file name) and a real path. Users cannot retype what they
// pasted; forcing them to memorize the id format is a product bug. A real repo
// had dropped one form: `replay fs_x.json` → "capsule not found: fs_x.json".
func TestCapsuleRefAcceptsIdNameAndPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "repeat-accumulate")
	store := filepath.Join(t.TempDir(), "c")
	if code, _, errOut := run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	entries, err := os.ReadDir(store)
	if err != nil || len(entries) == 0 {
		t.Fatalf("capsule was not written: %v", err)
	}
	name := entries[0].Name() // fs_xxx.json
	id := strings.TrimSuffix(name, ".json")

	for _, ref := range []string{id, name, filepath.Join(store, name)} {
		code, out, errOut := run("explain", "-capsule-dir", store, ref, "--format", "markdown")
		if code != domain.ExitOK {
			t.Errorf("ref %q: code=%d %s", ref, code, errOut)
			continue
		}
		if !strings.Contains(out, "| class | repeat |") {
			t.Errorf("ref %q: report missing:\n%s", ref, out)
		}
	}
}

// init does not overwrite an existing workflow: a file the team edited must not vanish.
func TestInitIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if code, _, errOut := run("init", dir); code != domain.ExitOK {
		t.Fatalf("code=%d %s", code, errOut)
	}
	path := filepath.Join(dir, ".github", "workflows", "flakesmith.yml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("workflow was not written: %v", err)
	}
	if err := os.WriteFile(path, append(before, []byte("# added by hand\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, _ := run("init", dir)
	if code != domain.ExitOK {
		t.Fatalf("second init code=%d", code)
	}
	if !strings.Contains(out, "untouched") {
		t.Errorf("out = %q", out)
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "added by hand") {
		t.Error("init overwrote the user's edit")
	}
}

// compile --verify proves the generated artifact really goes red.
//
// In the single-serial-template era repeat/parallel/env capsules produced green
// tests; this test stops that bug from returning class by class. The env class
// expects SKIP: the value is not in the capsule, the test must say so.
func TestCompileVerifyReproducesEachClass(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	cases := []struct {
		fixture string
		want    string
	}{
		{"order-basic", "RED"},
		{"repeat-accumulate", "RED"},
		{"parallel-race", "RED"},
		{"env-dependent", "SKIP"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			dir := fixtureDir(t, tc.fixture)
			store := filepath.Join(t.TempDir(), "c")

			if code, _, errOut := run("hunt", dir, "-capsule-dir", store,
				"--timeout", "150s", "--budget", "400"); code != domain.ExitOK {
				t.Fatalf("hunt code=%d %s", code, errOut)
			}
			code, out, errOut := run("compile", "-capsule-dir", store, "--pkg", dir, "--verify")
			if code != domain.ExitOK {
				t.Fatalf("compile --verify code=%d\n%s\n%s", code, out, errOut)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("verification should contain %q:\n%s", tc.want, out)
			}
		})
	}
}

// --format json must be a single document: an empty array on a clean run, two
// elements for two flakes. Consecutive pretty-printed objects are not JSON and
// no consumer can read them.
func TestHuntJSONIsOneDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	parse := func(t *testing.T, out string) map[string]any {
		t.Helper()
		var doc map[string]any
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		return doc
	}

	clean := fixtureDir(t, "clean")
	code, out, _ := run("hunt", clean, "--format", "json", "-capsule-dir", t.TempDir())
	if code != domain.ExitOK {
		t.Fatalf("clean hunt code=%d", code)
	}
	doc := parse(t, out)
	flakes, ok := doc["flakes"].([]any)
	if !ok {
		t.Fatalf("flakes is not an array: %s", out)
	}
	if len(flakes) != 0 {
		t.Errorf("flakes must be empty on a clean run: %s", out)
	}

	dirty := fixtureDir(t, "order-basic")
	code, out, _ = run("hunt", dirty, "--format", "json", "-capsule-dir", t.TempDir())
	if code != domain.ExitOK {
		t.Fatalf("hunt with flake code=%d", code)
	}
	doc = parse(t, out)
	flakes, _ = doc["flakes"].([]any)
	if len(flakes) == 0 {
		t.Errorf("flakes must not be empty: %s", out)
	}
}

// repro answers a single question: no answer means exit 3. That is the difference from hunt.
func TestReproNotReproducedExitsThree(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "clean")
	code, out, _ := run("repro", dir, "TestSum", "-capsule-dir", t.TempDir())
	if code != domain.ExitNotReproduced {
		t.Fatalf("code = %d, want %d\n%s", code, domain.ExitNotReproduced, out)
	}
}

// --race must NOT lower recall.
//
// Measured regression: when -race entered the search, race overhead pushed the
// timing witness down to 1/3 verification and the parallel-race fixture was
// never found with --race (exit 3), while it was found without race (exit 0).
// Since the evidence is collected in a separate run, both commands now find the
// same flake; only the label differs, and that label tells the truth.
func TestHuntRaceKeepsRecall(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "parallel-race")
	store := filepath.Join(t.TempDir(), "c")
	var out, errOut bytes.Buffer
	code := cli.Execute(context.Background(),
		[]string{"hunt", dir, "--race", "-capsule-dir", store},
		cli.IO{Stdout: &out, Stderr: &errOut})
	if code != domain.ExitOK {
		t.Fatalf("code = %d stderr=%s (race lowered recall)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "flake found: TestParallelBeta") {
		t.Fatalf("flake not found:\n%s", out.String())
	}

	entries, err := os.ReadDir(store)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one capsule: %v %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(store, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	// The label comes from the run's OWN result. This fixture is timing-based:
	// on a loaded machine the race run may fail to reproduce the witness. So the
	// claim is not "true will hold" but "flag and note will not contradict": if
	// it reproduced, race=true and no note; if not, false (the field drops via
	// omitempty) plus a one-line note.
	text := string(data)
	raced := strings.Contains(text, `"race": true`)
	noted := strings.Contains(text, "not reproduced under a race run")
	if raced == noted {
		t.Errorf("race label and note contradict (raced=%t noted=%t):\n%s", raced, noted, text)
	}
}

// compile's three surfaces: --print, --dry-run and the real write.
//
// The first two answer "what will be written", they are not a "write" command;
// touching the disk dirties the working tree while the user only wants a look.
// The third is called with an absolute --capsule-dir: if the capsule does not
// carry its own directory, the target package must come from the capsule, not
// from the process cwd — otherwise the test lands in flakesmith's own repo.
func TestCompilePrintDryRunAndWriteTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "order-basic")
	store := filepath.Join(t.TempDir(), "c")
	if code, _, errOut := run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	persistent := func(root string) []string {
		files, _ := filepath.Glob(filepath.Join(root, "*"+domain.PersistentFileSuffix))
		return files
	}

	code, out, errOut := run("compile", "-capsule-dir", store, "--print")
	if code != domain.ExitOK {
		t.Fatalf("--print code=%d %s", code, errOut)
	}
	if !strings.Contains(out, "func "+domain.PersistentFuncName("TestVictim")+"(") {
		t.Errorf("--print did not print the generated test:\n%s", out)
	}
	if left := persistent(dir); len(left) != 0 {
		t.Fatalf("--print wrote to disk: %v", left)
	}

	code, out, errOut = run("compile", "-capsule-dir", store, "--dry-run")
	if code != domain.ExitOK {
		t.Fatalf("--dry-run code=%d %s", code, errOut)
	}
	for _, want := range []string{"will write:", "command   :"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from --dry-run output:\n%s", want, out)
		}
	}
	if left := persistent(dir); len(left) != 0 {
		t.Fatalf("--dry-run wrote to disk: %v", left)
	}

	code, out, errOut = run("compile", "-capsule-dir", store)
	if code != domain.ExitOK {
		t.Fatalf("compile code=%d %s", code, errOut)
	}
	// The capsule stores the directory as the user typed it (relative), so the
	// written path prints relative too. The comparison uses absolute paths.
	want := filepath.Join(dir, "checkout"+domain.PersistentFileSuffix)
	line, _, _ := strings.Cut(strings.TrimPrefix(out, "regression test written: "), "\n")
	got, err := filepath.Abs(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("write target is not the capsule's package: %q, want %q", got, want)
	}
	if files := persistent(dir); len(files) != 1 {
		t.Errorf("expected a single regression test in the package: %v", files)
	}
	if left := persistent("."); len(left) != 0 {
		t.Errorf("written to cwd (the process directory was targeted): %v", left)
	}
}

// Positional directory: `compile <package>` and `ls .flakesmith` output use the
// same gate. With a directory argument the store is searched inside it.
func TestCompilePositionalPackageDir(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dir := fixtureDir(t, "order-basic")
	store := filepath.Join(dir, domain.CapsuleDir)
	if code, _, errOut := run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	code, out, errOut := run("compile", "--dry-run", dir)
	if code != domain.ExitOK {
		t.Fatalf("code=%d %s", code, errOut)
	}
	if !strings.Contains(out, "will write: "+filepath.Join(dir, "checkout"+domain.PersistentFileSuffix)) {
		t.Errorf("dry-run target is wrong:\n%s", out)
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
			dir := fixtureDir(t, tc.fixture)
			store := filepath.Join(t.TempDir(), "c")
			if code, _, errOut := run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
				t.Fatalf("hunt code=%d %s", code, errOut)
			}
			args := []string{"replay", "-capsule-dir", store, "-n", "2"}
			if tc.env != "" {
				args = append(args, "--env", tc.env)
			}
			code, out, errOut := run(args...)
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
			code, _, errOut := run(tc.args...)
			if code != domain.ExitUsage {
				t.Fatalf("code = %d, want %d (%s)", code, domain.ExitUsage, errOut)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("stderr %q should contain %q", errOut, tc.want)
			}
		})
	}
}
