package harness

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/emrelab/flakesmith/internal/domain"
)

// Harness is one installed witness file. Remove is safe to call twice.
type Harness struct {
	path   string
	digest string
	once   sync.Once
	err    error
}

// Digest is a short hash of the witness source this harness wrote.
//
// The runner hands it to the result cache as RunSpec.Source: two witness
// chains can share package, -run regex and flags while executing different
// subsets, and a cache that cannot tell them apart answers the wrong question.
func (h *Harness) Digest() string { return h.digest }

// New returns a harness bound to a package directory. It does not touch disk.
func New(pkgDir string) *Harness { return &Harness{path: filepath.Join(pkgDir, "")} }

// Install writes the witness file and returns its path.
//
// side suffix disambiguates concurrent installs in the same package (only one
// file may exist at a time; the go tool compiles the directory as a unit).
//
// CONTRACT: the caller holds the directory lock for the lifetime of the file,
// i.e. from Install until Remove. Locking only around the write is not enough —
// the bytes stay on disk while the caller builds, and a concurrent process that
// lists tests in that window sees a test that does not exist.
//
// The probe predicate is the intended owner (policy.Locked around
// Install → Run → Remove); Install deliberately does not take the lock itself
// because a nested acquisition would have to guess whether the caller already
// holds it.
func Install(pkgDir, clause, fn string, steps []domain.Step, side string) (*Harness, error) {
	return install(pkgDir, side, func() ([]byte, error) { return Render(clause, fn, steps) })
}

// InstallParallel installs a witness whose steps run as parallel subtests.
//
// Same contract as Install (caller holds the directory lock for the file's
// lifetime); the only difference is the template.
func InstallParallel(pkgDir, clause, fn string, steps []domain.Step, side string) (*Harness, error) {
	return install(pkgDir, side, func() ([]byte, error) { return RenderParallel(clause, fn, steps) })
}

func install(pkgDir, side string, build func() ([]byte, error)) (*Harness, error) {
	src, err := build()
	if err != nil {
		return nil, err
	}

	h := &Harness{
		path:   filepath.Join(pkgDir, domain.WitnessFileName(side)),
		digest: fmt.Sprintf("%x", sha256.Sum256(src))[:16],
	}
	if err := os.WriteFile(h.path, src, 0o644); err != nil {
		return nil, fmt.Errorf("%w: cannot write witness: %v", domain.ErrInternal, err)
	}
	return h, nil
}

// Path is the file written by Install.
func (h *Harness) Path() string { return h.path }

// Remove deletes the witness file. Missing file is not an error: cleanupStale
// and the deferred Remove may race, and both must stay idempotent.
func (h *Harness) Remove() error {
	h.once.Do(func() {
		if h.path == "" {
			return
		}
		if err := os.Remove(h.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			h.err = err
		}
	})
	return h.err
}

// CleanupStale deletes witness files left behind by a killed run.
//
// Called once at start-up: a crashed process cannot run its deferred Remove,
// and a stray witness file would make every later `go build` in that package
// fail with someone else's test errors.
func CleanupStale(pkgDir string) ([]string, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		if e.IsDir() || !domain.IsWitnessFileName(e.Name()) {
			continue
		}
		p := filepath.Join(pkgDir, e.Name())
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, p)
	}
	return removed, nil
}

// ResolveClause decides the package clause for a witness that calls the given
// test functions.
//
// Non-negotiable: every test in `names` must live in the same test package.
// `foo` and `foo_test` are two different binaries, so a witness in one cannot
// call a test from the other — the compiler says "undefined: TestX" and the
// user would see a confusing build error instead of our diagnosis.
func ResolveClause(clauseOf map[string]string, victim string, polluters []string) (string, error) {
	victimClause, ok := clauseOf[victim]
	if !ok {
		return "", fmt.Errorf("%w: %q is not a test function", domain.ErrUnsupportedTest, victim)
	}
	if victimClause == "" {
		return "", fmt.Errorf("%w: package clause of %q is unknown", domain.ErrUnsupportedTest, victim)
	}

	var mismatched []string
	for _, p := range polluters {
		c, ok := clauseOf[p]
		if !ok {
			return "", fmt.Errorf("%w: %q is not a test function", domain.ErrUnsupportedTest, p)
		}
		if c != victimClause {
			mismatched = append(mismatched, fmt.Sprintf("%s (package %s)", p, c))
		}
	}
	if len(mismatched) > 0 {
		return "", fmt.Errorf(
			"%w: %s (package %s) cannot be called in the same test binary as %s; "+
				"the two live in different test packages",
			domain.ErrUnsupportedTest, victim, victimClause, strings.Join(mismatched, ", "))
	}
	return victimClause, nil
}

// FuncName picks a witness function name that does not collide with a test the
// user already wrote (a package may legitimately have TestFlakeWitness).
func FuncName(existing map[string]string) string {
	base := domain.WitnessFuncName
	if _, taken := existing[base]; !taken {
		return base
	}
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		if _, taken := existing[candidate]; !taken {
			return candidate
		}
	}
	return base + "_x"
}
