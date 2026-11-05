package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
)

// gitRepo builds a throwaway repository with two test packages and then
// touches only the first one, so "changed" can be told apart from "everything
// in the repo".
//
// Tests never write into live testdata (rule: fixture copy); here the repo is
// built in t.TempDir because --changed's input is git history, not a file
// tree — a copied fixture has no history.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		// The user's global git identity and hooks must not affect the test.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=flakesmith", "GIT_AUTHOR_EMAIL=f@example.com",
			"GIT_COMMITTER_NAME=flakesmith", "GIT_COMMITTER_EMAIL=f@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-b", "main")
	write(filepath.Join(dir, "go.mod"), "module fixture/changed\n\ngo 1.26\n")
	one := filepath.Join(dir, "one", "one_test.go")
	write(one, `package one

import "testing"

func TestOne(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("arithmetic is broken")
	}
}
`)
	write(filepath.Join(dir, "two", "two_test.go"), `package two

import "testing"

func TestTwo(t *testing.T) {
	if 2+2 != 4 {
		t.Fatal("arithmetic is broken")
	}
}
`)
	git("add", "-A")
	git("commit", "-m", "two packages")

	// Only `one` changes: this is why the diff yields a single directory.
	write(one, `package one

import "testing"

func TestOne(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("arithmetic is broken")
	}
}

// comment added in the second commit
`)
	git("add", "-A")
	git("commit", "-m", "one changed")
	return dir
}

// --changed scans only the package in the diff and reports the count with --v.
//
// Without the count, "--changed scanned nothing" and "no package changed" fall
// into the same silent output; the user assumes the tool worked.
func TestHuntChangedScansOnlyChangedPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	repo := gitRepo(t)
	t.Chdir(repo)
	store := t.TempDir()

	// The second run has no --base: the repo has no remote, no origin/main, so
	// it must fall back to HEAD~1. A silent "everything" default would void
	// --changed's entire purpose.
	for _, args := range [][]string{
		{"hunt", "--changed", "--base", "HEAD~1", "-v", "-capsule-dir", store},
		{"hunt", "--changed", "-v", "-capsule-dir", store},
	} {
		code, stdout, stderr := run(args...)
		if code != domain.ExitOK {
			t.Fatalf("%v: code = %d (stdout=%s stderr=%s)", args, code, stdout, stderr)
		}
		if !strings.Contains(stderr, "packages to scan: 1") {
			t.Errorf("%v: scanned package count not visible:\n%s", args, stderr)
		}
		if !strings.Contains(stderr, "base HEAD~1") {
			t.Errorf("%v: base was not reported:\n%s", args, stderr)
		}
		if !strings.Contains(stderr, "  one\n") {
			t.Errorf("%v: changed package not listed:\n%s", args, stderr)
		}
		if strings.Contains(stderr, "  two\n") || strings.Contains(stdout, "TestTwo") {
			t.Errorf("%v: unchanged package was scanned (stdout=%s stderr=%s)", args, stdout, stderr)
		}
		if !strings.Contains(stdout, "TestOne") {
			t.Errorf("%v: changed package was not scanned:\n%s", args, stdout)
		}
	}
}

// A directory that is not a git repo: clear error and exit 2. A silent 0
// reads in CI as "--changed passed" and hides that nothing was scanned.
func TestHuntChangedOutsideGitIsUsageError(t *testing.T) {
	t.Chdir(t.TempDir())
	code, _, stderr := run("hunt", "--changed")
	if code != domain.ExitUsage {
		t.Fatalf("code = %d, want %d", code, domain.ExitUsage)
	}
	if !strings.Contains(stderr, "not a git repository") {
		t.Errorf("no explanation: %q", stderr)
	}
}

// A nonexistent base: the user is told instead of a guess being made.
func TestHuntChangedRejectsUnknownBase(t *testing.T) {
	repo := gitRepo(t)
	t.Chdir(repo)
	code, _, stderr := run("hunt", "--changed", "--base", "no-such-ref")
	if code != domain.ExitUsage {
		t.Fatalf("code = %d, want %d (stderr=%s)", code, domain.ExitUsage, stderr)
	}
	if !strings.Contains(stderr, "is not a valid git reference") {
		t.Errorf("no explanation: %q", stderr)
	}
}
