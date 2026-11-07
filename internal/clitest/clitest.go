// Package clitest holds the shared harness for the CLI's integration tests.
//
// Why a package instead of a helper file inside internal/cli: those tests are
// split across several test packages (hunt, huntci, huntevidence, compile,
// replay, surface) so that `go test ./...` can schedule them in parallel. Each
// test spawns real `go test` subprocesses, so wall time is the whole reason the
// split exists; a helper file in one package cannot be reached from the others,
// and a copy per package would drift.
package clitest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/cli"
	"github.com/emrelab/flakesmith/internal/domain"
)

// Run executes the CLI in-process and returns exit code, stdout and stderr.
func Run(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = cli.Execute(context.Background(), args, cli.IO{Stdout: &o, Stderr: &e})
	return code, o.String(), e.String()
}

// FixtureDir copies testdata/fixtures/<name> into a fresh temp dir and returns
// it. Tests never run hunt against the fixture in place: hunt writes witness
// files, capsules and regression tests, and a shared fixture directory would
// make two test packages race over the same files.
//
// The source path is resolved from this file's own location, not from the
// working directory: after the split, the working directory is the test
// package (internal/cli/hunt, ...), which is a different depth than the one
// this helper used to live at.
func FixtureDir(t testing.TB, name string) string {
	t.Helper()
	src := filepath.Join(repoRoot(t), "testdata", "fixtures", name)
	dst := t.TempDir()

	err := filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
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
		t.Fatalf("could not copy fixture %q from %s: %v", name, src, err)
	}
	return dst
}

// StoredVerifyStatus reads verifyStatus back from the newest capsule file.
func StoredVerifyStatus(t testing.TB, store string) domain.VerifyStatus {
	t.Helper()
	ids, err := capsule.NewStore(store).List()
	if err != nil || len(ids) == 0 {
		t.Fatalf("no capsule in store: %v", err)
	}
	c, err := capsule.NewStore(store).Read(ids[0])
	if err != nil {
		t.Fatal(err)
	}
	return c.VerifyStatus
}

// repoRoot finds the module root by walking up from this source file. Under
// -trimpath the caller path is module-relative and the walk finds nothing, so
// it falls back to walking up from the working directory (the test package).
func repoRoot(t testing.TB) string {
	t.Helper()
	var starts []string
	if _, file, _, ok := runtime.Caller(0); ok {
		starts = append(starts, filepath.Dir(file))
	}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			if parent := filepath.Dir(dir); parent == dir {
				break
			}
		}
	}
	t.Fatalf("could not locate the module root from %v", starts)
	return ""
}
