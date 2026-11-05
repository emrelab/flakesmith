package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/gotest"
)

// defaultBase is what `--changed` measures against when the user names nothing:
// the branch a pull request is diffed against. A clone without a remote has no
// origin/main, so resolveBase falls back to HEAD~1 (see there).
const defaultBase = "origin/main"

// changedPackages derives the package directories the diff base...HEAD touches.
//
// Only .go files count: changing a README or a CI file does not change test
// flake behavior, and scanning an unaffected package voids --changed's only
// promise — cutting wall clock. A deleted file's directory is already gone;
// os.Stat filters it out.
func changedPackages(ctx context.Context, cwd, base string) ([]string, string, error) {
	if _, err := gitOutput(ctx, cwd, "rev-parse", "--git-dir"); err != nil {
		// git's own "fatal: not a git repository" text does not tell the user
		// what to do: we write why the flag needs it.
		return nil, "", domain.Wrap(domain.ErrUsage,
			"%s is not a git repository (--changed reads git history)", cwd)
	}

	used, err := resolveBase(ctx, cwd, base)
	if err != nil {
		return nil, "", err
	}

	// --relative: paths come out relative to cwd, so a run invoked from a
	// subdirectory does not scan the wrong dir via repo-root-relative paths.
	files, err := gitOutput(ctx, cwd, "diff", "--relative", "--name-only", used+"...HEAD")
	if err != nil {
		return nil, "", domain.Wrap(domain.ErrUsage, "git diff %s...HEAD failed: %v", used, err)
	}

	seen := map[string]bool{}
	var dirs []string
	for _, line := range strings.Split(files, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ".go") {
			continue
		}
		dir := filepath.Dir(filepath.Join(cwd, line))
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		key := canonical(dir)
		if seen[key] {
			continue
		}
		seen[key] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		return nil, used, nil
	}

	// A directory without tests is not a package to scan. The set rule lives
	// in one place, so the call goes to gotest; if everything is filtered out,
	// ExpandPatterns returns ErrUsage, which here means "nothing to scan".
	resolved, err := gotest.ExpandPatterns(ctx, dirs)
	if err != nil {
		if errors.Is(err, domain.ErrUsage) {
			return nil, used, nil
		}
		return nil, "", err
	}
	return resolved, used, nil
}

// resolveBase picks the reference the diff is taken from.
//
// Default is origin/main; a repo without a remote has no such ref, then the
// comparison falls back to the last commit. A single-commit repo has neither,
// and the user is asked for --base instead of guessing.
func resolveBase(ctx context.Context, cwd, base string) (string, error) {
	if base != "" {
		if !hasCommit(ctx, cwd, base) {
			return "", domain.Wrap(domain.ErrUsage,
				"%q is not a valid git reference (--base)", base)
		}
		return base, nil
	}
	if hasCommit(ctx, cwd, defaultBase) {
		return defaultBase, nil
	}
	if hasCommit(ctx, cwd, "HEAD~1") {
		return "HEAD~1", nil
	}
	return "", domain.Wrap(domain.ErrUsage,
		"%s is missing and so is HEAD~1 (single-commit repo); give a base with --base", defaultBase)
}

func hasCommit(ctx context.Context, cwd, ref string) bool {
	_, err := gitOutput(ctx, cwd, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// restrictToArgs intersects the changed set with the packages the user named,
// so `hunt ./... --changed` means "what changed in this tree" instead of silently
// ignoring the argument. Naming nothing means the whole repository.
func restrictToArgs(ctx context.Context, changed, pkgs []string) ([]string, error) {
	if len(pkgs) == 0 {
		return changed, nil
	}
	allowed, err := gotest.ExpandPatterns(ctx, pkgs)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(allowed))
	for _, d := range allowed {
		set[canonical(d)] = true
	}
	var out []string
	for _, d := range changed {
		if set[canonical(d)] {
			out = append(out, d)
		}
	}
	return out, nil
}

// canonical resolves symlinks so two spellings of one directory (macOS
// /var -> /private/var, a symlinked checkout) compare equal.
func canonical(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return dir
}
