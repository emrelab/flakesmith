package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/compile"
	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/facts"
	"github.com/emrelab/flakesmith/internal/gotest"
	"github.com/emrelab/flakesmith/internal/harness"
)

// compileCmd turns one capsule (or the newest in the store) into a permanent
// regression test in the package it belongs to.
func compileCmd(ctx context.Context, args []string, out IO) error {
	var (
		dir    string
		pkg    string
		print  bool
		dryRun bool
		verify bool
	)
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	fs.SetOutput(out.Stderr)
	fs.StringVar(&dir, "capsule-dir", "", "capsule directory (default <pkg>/.flakesmith)")
	fs.StringVar(&pkg, "pkg", "", "package directory (default: the directory the capsule recorded)")
	fs.BoolVar(&print, "print", false, "print to stdout without writing to disk")
	fs.BoolVar(&dryRun, "dry-run", false, "only say what would be written")
	fs.BoolVar(&verify, "verify", false, "run the generated test once, verify it really goes red")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	id, storeDir, pkgDir, err := resolveCapsule(fs.Args(), dir, pkg)
	if err != nil {
		return err
	}

	store := capsule.NewStore(storeDir)
	c, err := store.Read(id)
	if err != nil {
		return domain.Wrap(domain.ErrUsage, "%s", err)
	}

	// The capsule is portable: if it does not carry its own directory, find
	// the package hosting the victim. Otherwise compile called with
	// `--capsule-dir /abs/path` would look for tests in cwd (flakesmith's repo).
	pkgDir, err = resolvePkgDir(pkgDir, c)
	if err != nil {
		return err
	}

	// Clause determines which test package the generated test belongs to.
	f, err := facts.Scan(pkgDir)
	if err != nil {
		return domain.Wrap(domain.ErrUsage, "could not read %s", pkgDir)
	}
	clause, err := harness.ResolveClause(f.Clause, c.Witness.Victim.Name, nil)
	if err != nil {
		return err
	}

	if print {
		src, err := compile.Render(c, clause)
		if err != nil {
			return err
		}
		if _, err := out.Stdout.Write(src); err != nil {
			return err
		}
		return nil
	}

	if dryRun {
		_, _ = fmt.Fprintf(out.Stdout, "will write: %s\n", filepath.Join(pkgDir, domain.PersistentFileName(sourceFile(pkgDir, c.Witness.Victim.Name))))
		_, _ = fmt.Fprintf(out.Stdout, "command   : %s\n", compile.CommandLine(c))
		return nil
	}

	path, err := compile.Write(c, clause, compile.Options{
		Dir:        pkgDir,
		SourceFile: sourceFile(pkgDir, c.Witness.Victim.Name),
		Write:      true,
	})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out.Stdout, "regression test written: %s\n", path)
	_, _ = fmt.Fprintf(out.Stdout, "run              : %s\n", compile.CommandLine(c))

	if verify {
		if err := verifyCompiled(ctx, pkgDir, c, out); err != nil {
			return err
		}
	}

	if c.Kind == domain.KindEnv {
		_, _ = fmt.Fprintf(out.Stdout, "\nNote: the test turns RED when the env value is given (the value is not stored in the capsule):\n")
		_, _ = fmt.Fprintf(out.Stdout, "     %s\n", compile.CommandLine(c))
	} else {
		_, _ = fmt.Fprintf(out.Stdout, "\nNote: the test will be RED right now (the flake is not fixed yet).\n")
		_, _ = fmt.Fprintf(out.Stdout, "     After the fix it turns green and guards the regression permanently.\n")
	}
	return nil
}

// verifyCompiled runs the artifact once, exactly as the user would.
//
// Without it `compile` only asserts a shape: the serial template produced a
// green test for repeat, parallel and env capsules while still printing "the
// test will be RED now". A verification that fails is not a warning — an
// artifact that does not reproduce the flake is worthless, and the exit code
// must say so.
func verifyCompiled(ctx context.Context, dir string, c domain.Capsule, out IO) error {
	res, err := (&gotest.Exec{}).Run(ctx, compile.VerifySpec(c, dir))
	if err != nil {
		return err
	}
	fn := domain.PersistentFuncName(c.Witness.Victim.Name)

	switch {
	case res.BuildFailed:
		return domain.Wrap(domain.ErrBuildFailed, "the generated regression test did not compile: %s",
			firstBuildLine(res.BuildOutput))
	case res.Failed(fn):
		_, _ = fmt.Fprintln(out.Stdout, "verification     : RED (expected — the flake is still there)")
		return nil
	case res.Status(fn) == domain.StatusSkip:
		// The env class has no value in the capsule; the test says so with SKIP.
		_, _ = fmt.Fprintln(out.Stdout, "verification     : SKIP (no env value given; run with --env)")
		return nil
	default:
		return domain.Wrap(domain.ErrNotReproducible,
			"the generated test stayed green: for class %s the artifact does not reproduce the flake", c.Kind)
	}
}

// firstBuildLine keeps a compiler diagnostic on one line.
func firstBuildLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "no detail"
}

// resolveCapsule turns the command's positional argument into a capsule id and
// the two directories that matter: where the JSON lives and which package the
// capsule is about.
//
// They are not the same directory. `replay .flakesmith/fs_x.json` used to take
// the file's parent as the package, then look for its store inside that parent
// (<pkg>/.flakesmith/.flakesmith) and report "capsule not found" for a file
// sitting right there.
func resolveCapsule(args []string, dirFlag, pkgFlag string) (id, storeDir, pkgDir string, err error) {
	pkg := pkgOrDefault(pkgFlag, ".")

	switch {
	case len(args) > 1:
		return "", "", "", domain.Wrap(domain.ErrUsage, "at most one capsule")
	case len(args) == 1:
		arg := args[0]
		if st, statErr := os.Stat(arg); statErr == nil && !st.IsDir() {
			// A path to a capsule file: the store is its directory.
			store := filepath.Dir(arg)
			return stemExt(arg), store, pkgOrDefault(pkgFlag, filepath.Dir(store)), nil
		}
		if st, statErr := os.Stat(arg); statErr == nil && st.IsDir() {
			id, pkgDir, err = newestIn(capsuleDir(dirFlag, arg), arg)
			return id, capsuleDir(dirFlag, pkgDir), pkgDir, err
		}
		// Bare file name: whatever `ls .flakesmith` prints gets pasted.
		// The .json extension is not part of the id; look it up as an id. If no
		// directory is given, the store comes from --capsule-dir, not from cwd.
		if filepath.Ext(arg) == ".json" {
			if d := filepath.Dir(arg); d != "." {
				return stemExt(arg), d, pkgOrDefault(pkgFlag, filepath.Dir(d)), nil
			}
			return stemExt(arg), capsuleDir(dirFlag, pkg), pkg, nil
		}
		return arg, capsuleDir(dirFlag, pkg), pkg, nil
	}
	id, pkgDir, err = newestIn(capsuleDir(dirFlag, pkg), pkg)
	return id, capsuleDir(dirFlag, pkgDir), pkgDir, err
}

func pkgOrDefault(pkg, fallback string) string {
	if pkg != "" {
		return pkg
	}
	return fallback
}

// resolvePkgDir picks the package directory to write into.
//
// A capsule carries Target.RelDir so it can travel in a PR comment or a CI
// artifact. When the caller only names the store (--capsule-dir), the package
// has to come from the capsule, not from the process cwd — otherwise compile
// writes the regression test into whatever repository it happens to run in.
//
// "Exists" is therefore not enough to pick a candidate: "." exists in every
// checkout, and it used to win before Target.RelDir was ever looked at. A
// candidate wins by declaring the victim; the first existing directory is only
// the fallback, so the complaint stays "test format" rather than "package not found".
func resolvePkgDir(pkgDir string, c domain.Capsule) (string, error) {
	candidates := []string{pkgDir, c.Target.RelDir, "."}
	fallback := ""
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if st, err := os.Stat(cand); err != nil || !st.IsDir() {
			continue
		}
		if fallback == "" {
			fallback = cand
		}
		if f, err := facts.Scan(cand); err == nil {
			if _, ok := f.Clause[c.Witness.Victim.Name]; ok {
				return cand, nil
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", domain.Wrap(domain.ErrUsage,
		"package containing test %s not found; specify it with --pkg", c.Witness.Victim.Name)
}

func newestIn(dir, pkgDir string) (string, string, error) {
	ids, err := capsule.NewStore(dir).List()
	if err != nil {
		return "", "", domain.Wrap(domain.ErrUsage, "%s", err)
	}
	if len(ids) == 0 {
		return "", "", domain.Wrap(domain.ErrUsage, "no capsule in %s; run flakesmith hunt first", dir)
	}
	return ids[0], pkgDir, nil
}

func stemExt(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// sourceFile names the file the victim lives in; the generated file is named
// after it. The victim is passed so the name follows the test, not directory
// order, and so flakesmith's own output is never picked again.
func sourceFile(dir, victim string) string { return targetSourceFile(dir, victim) }
