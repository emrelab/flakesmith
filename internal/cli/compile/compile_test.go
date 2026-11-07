package clicompile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/clitest"
	"github.com/emrelab/flakesmith/internal/domain"
)

// compile --verify proves the generated artifact really goes red.
//
// In the single-serial-template era repeat/parallel/env capsules produced green
// tests; this test stops that bug from returning class by class. The env class
// is the reproduction contract (#8): with the variable absent the witness is
// inert — that is NOT verification, so the exit code is 3 and the capsule says
// skipped-missing-env. With the variable present it must go RED and record
// verified.
func TestCompileVerifyReproducesEachClass(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	cases := []struct {
		fixture string
		env     string // KEY=VALUE set for the verify run ("" = leave unset)
		want    string
		code    int
	}{
		{"order-basic", "", "RED", domain.ExitOK},
		{"repeat-accumulate", "", "RED", domain.ExitOK},
		{"parallel-race", "", "RED", domain.ExitOK},
		{"env-dependent", "", "unverified", domain.ExitNotReproduced},
		{"env-dependent", "FIXTURE_MODE=ci", "RED", domain.ExitOK},
	}
	for _, tc := range cases {
		name := tc.fixture
		if tc.env != "" {
			name += "+env"
		}
		t.Run(name, func(t *testing.T) {
			dir := clitest.FixtureDir(t, tc.fixture)
			store := filepath.Join(t.TempDir(), "c")

			if code, _, errOut := clitest.Run("hunt", dir, "-capsule-dir", store,
				"--timeout", "150s", "--budget", "400"); code != domain.ExitOK {
				t.Fatalf("hunt code=%d %s", code, errOut)
			}
			if tc.env != "" {
				t.Setenv(strings.SplitN(tc.env, "=", 2)[0], strings.SplitN(tc.env, "=", 2)[1])
			}
			code, out, errOut := clitest.Run("compile", "-capsule-dir", store, "--pkg", dir, "--verify")
			if code != tc.code {
				t.Fatalf("compile --verify code=%d, want %d\n%s\n%s", code, tc.code, out, errOut)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("verification should contain %q:\n%s", tc.want, out)
			}
			// The verdict must survive in the capsule, not just in stdout:
			// downstream tooling binds to verifyStatus, never to exit codes.
			wantStatus := domain.VerifyVerified
			if tc.code != domain.ExitOK {
				wantStatus = domain.VerifySkippedMissingEnv
			}
			if st := clitest.StoredVerifyStatus(t, store); st != wantStatus {
				t.Errorf("capsule verifyStatus = %q, want %q", st, wantStatus)
			}
		})
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
	dir := clitest.FixtureDir(t, "order-basic")
	store := filepath.Join(t.TempDir(), "c")
	if code, _, errOut := clitest.Run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	persistent := func(root string) []string {
		files, _ := filepath.Glob(filepath.Join(root, "*"+domain.PersistentFileSuffix))
		return files
	}

	code, out, errOut := clitest.Run("compile", "-capsule-dir", store, "--print")
	if code != domain.ExitOK {
		t.Fatalf("--print code=%d %s", code, errOut)
	}
	if !strings.Contains(out, "func "+domain.PersistentFuncName("TestVictim")+"(") {
		t.Errorf("--print did not print the generated test:\n%s", out)
	}
	if left := persistent(dir); len(left) != 0 {
		t.Fatalf("--print wrote to disk: %v", left)
	}

	code, out, errOut = clitest.Run("compile", "-capsule-dir", store, "--dry-run")
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

	code, out, errOut = clitest.Run("compile", "-capsule-dir", store)
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
	dir := clitest.FixtureDir(t, "order-basic")
	store := filepath.Join(dir, domain.CapsuleDir)
	if code, _, errOut := clitest.Run("hunt", dir, "-capsule-dir", store, "--format", "json"); code != domain.ExitOK {
		t.Fatalf("hunt code=%d %s", code, errOut)
	}
	code, out, errOut := clitest.Run("compile", "--dry-run", dir)
	if code != domain.ExitOK {
		t.Fatalf("code=%d %s", code, errOut)
	}
	if !strings.Contains(out, "will write: "+filepath.Join(dir, "checkout"+domain.PersistentFileSuffix)) {
		t.Errorf("dry-run target is wrong:\n%s", out)
	}
}
