# FlakeSmith — Architecture and Coding Plan

**Version:** 1.1 · **Date:** November 1, 2026 · **Last synced:** November 4, 2026
**Status:** Implemented — Stages 1–9 and automation rounds A0–A6 are in the code; the only open item is publishing the repo (push + `v0.1.0` tag).

This document describes not "what we will do" but **how we will build it**: package boundaries, types, algorithms, Go-specific traps, and task order. The product definition is not repeated here.

---

## 1. One-sentence architecture decision

**FlakeSmith is not a runtime — it is an orchestrator that drives `go test`.**

It runs processes, reads `-json`, **generates a temporary test file** to change the order, shrinks the result, and writes a permanent regression test. At no stage does it patch the Go toolchain, fork a runtime, or compile to WASM.

This decision is the single thing that fits the MVP into 8–10 weeks. ReplayGo (the A− candidate) was eliminated for exactly this reason: touching the runtime means a rebase on every Go release.

### 1.1. Five design constraints

| # | Constraint | Why | If violated |
|---|---|---|---|
| K1 | **Zero external dependencies** (stdlib only in the MVP) | `go install` gives one binary; zero supply-chain surface | `go.sum` grows, review burden rises, the SLSA story weakens |
| K2 | **Never modify source code** — only add/remove *new* files | The user's `git diff` must not get dirty; trust threshold | The tool gets stamped "it changed my code" and is abandoned |
| K3 | **Runner is an interface** — the layer above doesn't know `go test` | Being testable without real flakes | Our own tests go flaky in CI, development stalls |
| K4 | **Every run is budgeted and cancellable** | Not hanging 40 minutes on a pathological search | Eats CI timeouts, users get used to `Ctrl-C` |
| K5 | **Evidence comes from observation; no "root cause" is claimed** | A wrong diagnosis kills trust in the tool | The user fixes the wrong place, the product gets blamed |

---

## 2. Layers

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/flakesmith         main() → cli.Run() → os.Exit(code)  │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│  internal/cli        argument → command → report → exit code │
│  (hunt / repro / compile / replay / explain)                 │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│  internal/probe         hypothesis order: repeat→order→      │
│                         parallel→env                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  internal/search   ddmin + verified minimization      │   │
│  └──────────────────────────────────────────────────────┘   │
└────┬──────────────┬──────────────┬──────────────┬───────────┘
     │              │              │              │
┌────▼─────┐ ┌──────▼──────┐ ┌─────▼──────┐ ┌────▼─────────┐
│ gotest   │ │  harness    │ │  facts     │ │  evidence    │
│ (Runner) │ │ (gen/del)   │ │  (AST)     │ │  (env/fs/…)  │
└────┬─────┘ └──────┬──────┘ └─────┬──────┘ └────┬─────────┘
     │              │              │              │
     │       ┌──────▼──────────────▼──────────────▼───────┐
     │       │  internal/capsule        permanent artifact │
     │       └──────────────────┬──────────────────────────┘
     │                          │
┌────▼──────────────────────────▼─────────────────────────────┐
│  internal/compile     capsule → *_flakesmith_test.go        │
└─────────────────────────────────────────────────────────────┘

  Cross-cutting: internal/domain (types) · internal/policy (budget, redaction)
                 internal/report (text|json|markdown)
```

**Dependency direction is one-way:** `domain` imports nothing. `gotest`, `facts`, `harness`, `evidence` use only `domain` + stdlib. `probe` uses those. `cli` is on top. No reverse dependencies — nothing looks at `domain` except from its own `domain_test` package.

---

## 3. Directory structure

```
flakesmith/
├── go.mod                          module github.com/emrelab/flakesmith
├── LICENSE                         Apache-2.0
├── README.md
├── cmd/flakesmith/main.go          ~15 lines: no flags, only cli.Run
├── internal/
│   ├── domain/                     ★ single source of truth: types, constants, errors
│   │   ├── test.go                 TestID, Status, Outcome
│   │   ├── run.go                  RunSpec, RunResult, Flags
│   │   ├── witness.go              Kind, Witness, Step
│   │   ├── capsule.go              Capsule + schema version
│   │   ├── names.go                generated file/function names (one place)
│   │   └── errors.go               typed errors + exit codes
│   ├── gotest/
│   │   ├── runner.go               real implementation of the Runner interface (exec)
│   │   ├── discover.go             go test -list
│   │   ├── json.go                 test2json events → RunResult
│   │   └── normalize.go            output → stable signature (strip time/tmp/addr)
│   ├── facts/
│   │   ├── scan.go                 go/ast + go/parser walk
│   │   └── facts.go                PkgFacts (static fact sheet)
│   ├── harness/
│   │   ├── template.go             text template
│   │   ├── render.go               render + go/format validation
│   │   └── install.go              write / delete / stale cleanup / signal capture
│   ├── search/
│   │   ├── ddmin.go                classic ddmin
│   │   └── minimize.go             ddmin + re-verify N times
│   ├── probe/
│   │   ├── probe.go                Probe interface, Target
│   │   ├── repeat.go               -count=N (cheapest)
│   │   ├── order.go                order dependency
│   │   ├── parallel.go             t.Parallel / shared resource
│   │   ├── env.go                  os.Getenv effect
│   │   └── registry.go             list by cost order
│   ├── evidence/
│   │   ├── env.go                  environment diff
│   │   ├── fs.go                   file/port observation
│   │   ├── static.go               AST: package var read/write
│   │   └── race.go                 -race output summary
│   ├── capsule/
│   │   ├── id.go                   content-addressed identity
│   │   └── store.go                .flakesmith/ directory
│   ├── compile/
│   │   ├── persistent.go           build-tagged permanent test generation
│   │   └── golden.go               golden file comparison (test helper)
│   ├── policy/
│   │   ├── budget.go               run/time budget
│   │   └── redact.go               env value masking
│   └── report/
│       ├── text.go
│       ├── json.go
│       └── markdown.go             PR comment output
├── testdata/
│   ├── fixtures/                   mini modules with their own go.mod
│   │   ├── order-basic/            shared global
│   │   ├── parallel-race/          t.Parallel + shared counter
│   │   ├── env-dependent/          os.Getenv
│   │   ├── file-leak/              temp file
│   │   ├── repeat-accumulate/      state accumulated across -count
│   │   └── clause-mismatch/        package foo vs foo_test
│   └── golden/                     expected generated test files
└── .github/workflows/ci.yml        linux+macos, -race, fixtures
```

`testdata/` is skipped by the go tool — the fixtures don't pollute `./...`, each one is its own module.

---

## 4. Domain types (single source of truth)

No package outside `internal/domain` defines its own `TestID`/`RunSpec`-like type.

```go
package domain

// TestID identifies a test function together with its package.
// Name is always an exported function prefixed with "Test".
type TestID struct {
	Package string // import path: "example.com/svc/internal/checkout"
	Name    string // "TestCheckout"
}

func (t TestID) String() string { return t.Package + "." + t.Name }

type Status uint8

const (
	StatusSkip Status = iota
	StatusPass
	StatusFail
)

// Outcome is a single test's result in a single run.
type Outcome struct {
	Status  Status
	Elapsed time.Duration
	Output  string // normalized; not raw output
}

// RunSpec is the full description of a "go test" invocation. Same spec + same commit = same run.
type RunSpec struct {
	Package     string        // used as the working directory
	RunRegex    string        // "" → all tests ("^TestX$" for an exact match)
	Count       int           // always >= 1; 1 also breaks the test cache
	Parallel    int           // -parallel; 0 → leave alone
	CPU         int           // -cpu; 0 → leave alone
	ShuffleSeed int64         // 0 → -shuffle off
	Race        bool          // -race
	Env         []string      // "KEY=VALUE" layer; nil → inherit
	Tags        []string      // -tags; changes the set of compiled files
	Timeout     time.Duration // 0 → DefaultTimeout (120s)
	Source      string        // identity of the generated witness file; never reaches the go command
}

// RunResult is the observation of one run.
type RunResult struct {
	Spec        RunSpec
	Outcomes    map[string]Outcome // test name → outcome (subtests not included)
	Order       []string           // order of the -json "run" events
	BuildFailed bool               // build error != test error
	BuildOutput string
	ExitCode    int
	Duration    time.Duration
	TimedOut    bool
}

// Kind is the flake class: the output of the four probes in docs/ARCHITECTURE.md §7.
type Kind string

const (
	KindRepeat   Kind = "repeat"   // fails when repeated in the same process
	KindOrder    Kind = "order"    // fails when another test runs first
	KindParallel Kind = "parallel" // fails only when run in parallel
	KindEnv      Kind = "env"      // depends on an environment variable
)

// Step is a single t.Run call inside the harness.
type Step struct {
	Label string // subtest name: "polluter/TestSeedDB"
	Test  TestID
	// WrapParallel: the witness calls t.Parallel() for tests that don't call it
	// themselves; otherwise the steps run serially and the parallel witness proves
	// nothing. A test that calls its own t.Parallel() is left alone: the second call panics.
	WrapParallel bool
}

// Witness is the minimal witness found.
type Witness struct {
	Kind      Kind
	Victim    TestID
	Polluters []TestID
	Cleaners  []TestID
	Steps     []Step // the exact order to be written into the harness
	Flags     Flags  // flags needed for reproduction
	Seed      int64  // -shuffle seed for KindOrder
	EnvDiff   []string
	Runs      int    // verified over how many runs
}

type Flags struct {
	Parallel int  `json:"parallel,omitempty"`
	Race     bool `json:"race,omitempty"`
	// Count is the -count value a repeat witness was found with ("--repeat N"). Without it
	// a capsule found with -count=50 replays with the verification count (2), goes
	// green, and the tool reports "flake fixed" — a false diagnosis that kills trust
	// in every later capsule.
	Count int `json:"count,omitempty"`
}
```

`RunSpec.Source` never reaches the go command; it only separates the result cache.
Without it, every ddmin subset asks the same spec (same package, same `-run`, same
flags), one subset's PASS is served to the next, and the search eliminates the real
polluter. `Tags` is what lets `compile --verify` run with `-tags flakesmith`:
a spec without the build tag does not see the generated test.

The flag set a witness writes into a capsule: `Flags{Parallel: 2, Race: true, Count: 10}`
— all three flow into the reproduction command and the `compile --verify` spec; `Count`
is only filled for a repeat witness.

`domain/names.go` — every generated name lives in one place (K3 and the single-source-of-truth rule):

```go
package domain

const (
	// Temporary harness. Files starting with "_" or "." are SKIPPED by the go
	// tool — that's why the "zz_" prefix is required (verified on November 1, 2026).
	WitnessFilePrefix = "zz_flakesmith_witness_"

	// Permanent artifact.
	PersistentFileSuffix = "_flakesmith_test.go"
	PersistentBuildTag   = "flakesmith"

	// Function name inside the harness; a suffix is added on collision.
	WitnessFuncName = "TestFlakeWitness"

	// Capsule directory (relative to the repo root).
	CapsuleDir = ".flakesmith"
)
```

---

## 5. Runner: the single real-world touchpoint

This is the only place in the whole system that sees `go test`. K3 made concrete:

```go
package gotest

// Runner is the only interface the probe/search layers see.
// Tests replace it with FakeRunner; the layer above knows nothing about exec.
type Runner interface {
	Discover(ctx context.Context, pkg string) ([]domain.TestID, error)
	Run(ctx context.Context, spec domain.RunSpec) (*domain.RunResult, error)
}
```

### 5.1. Discover

`go test -list '.*'` gives source order (verified). Subtests do not appear here — outside MVP scope; they are detected via `PkgFacts.Subtests` and `ErrUnsupportedTest` is returned.

```go
func (r *Exec) Discover(ctx context.Context, pkg string) ([]domain.TestID, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-list", `.*`, ".")
	cmd.Dir = pkg
	cmd.Env = os.Environ() // GOFLAGS, GOCACHE, PATH are inherited — never a clean env
	out, err := cmd.Output()
	// "ok  \tpkg\t0.12s" lines are dropped; every remaining line is a test name
}
```

### 5.2. Run

```go
func (r *Exec) Run(ctx context.Context, spec domain.RunSpec) (*domain.RunResult, error) {
	if spec.Count < 1 {
		spec.Count = 1 // -count=1 is mandatory: the test cache hides flakes
	}
	args := []string{"test", "-json", "-count", strconv.Itoa(spec.Count)}
	if spec.RunRegex != "" {
		args = append(args, "-run", spec.RunRegex)
	}
	if spec.Parallel > 0 {
		args = append(args, "-parallel", strconv.Itoa(spec.Parallel))
	}
	if spec.CPU > 0 {
		args = append(args, "-cpu", strconv.Itoa(spec.CPU))
	}
	if spec.ShuffleSeed != 0 {
		args = append(args, "-shuffle", strconv.FormatInt(spec.ShuffleSeed, 10))
	}
	if spec.Race {
		args = append(args, "-race")
	}
	args = append(args, ".")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = spec.Package
	cmd.Env = append(os.Environ(), spec.Env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err := cmd.Run()
	// *exec.ExitError is expected: a failing test is data, not an error.
	return parseJSON(stdout.Bytes(), spec, err, time.Since(start))
}
```

**Critical distinctions:**
- `*exec.ExitError` → don't return an error, write to `RunResult.ExitCode`. A failing test is the normal flow.
- Build failure (`[build failed]` + build output) → `BuildFailed=true`; this is **not** a flake, it's a user error. No budget is spent; the probe chain stops immediately.
- `context.DeadlineExceeded` → `TimedOut=true`; a timeout may be the *cause* of the flake — not a separate hypothesis, an evidence field.
- `cmd.Dir = spec.Package` + pattern `.` was chosen: `go.work`, `GOFLAGS`, vendor all resolve correctly on their own. The price: paths in the output are relative to the package directory and must be converted to absolute paths.

### 5.3. Order reconstruction

In the `-json` stream, `Action: "run"` events give the **actual start order** of the tests (verified). But in parallel tests, the `run` event of a test calling `t.Parallel()` happens at the moment of *pause*, i.e. the order is misleading.

**That's why the order-dependency hypothesis is only searched under `-parallel=1`.** A parallel flake is a separate hypothesis (`KindParallel`); its minimization is done not over an order but over a **set**.

```go
// parseJSON → RunResult.Order: only Action=="run" + Test!="" events,
// in arrival order. Subtests ("TestA/sub") are dropped.
```

### 5.4. Output normalization

The same error arrives with a different timestamp/tmp path/pointer in two runs; the signature comparison must strip them:

```go
var normalizers = []*regexp.Regexp{
	regexp.MustCompile(`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(\.\d+)?`), // timestamp
	regexp.MustCompile(`/var/folders/\S+`),                              // macOS tmp
	regexp.MustCompile(`/tmp/\S+`),                                      // linux tmp
	regexp.MustCompile(`0x[0-9a-f]{6,}`),                                // pointer
	regexp.MustCompile(`\b\d+(\.\d+)?(ns|µs|ms|s)\b`),                   // duration
	regexp.MustCompile(`127\.0\.0\.1:\d+`),                              // ephemeral port
}
```

---

## 6. Harness: the heart of the product

**Single generator, two consumers.** Temporary during the search, permanent at the end. That's why the search and the artifact are the *same mechanism* — the code-level counterpart of the "Go-native engine + compiled test" defense.

### 6.1. Why must we generate?

Verified fact: `go test -run '^(TestAlpha|TestZebra)$'` runs in **source order**; the alternation order in the regex changes nothing. The only way to put tests in order in Go is to write a test that calls them.

```go
const witnessTmpl = `// Code generated by flakesmith; DO NOT EDIT.
// Temporary witness file — deleted after the run.

package {{.Package}}

import "testing"

func {{.Func}}(t *testing.T) {
{{- range .Steps}}
	t.Run({{printf "%q" .Label}}, {{.Test}})
{{- end}}
}
`
```

Generated example (for the `order-basic` fixture):

```go
// Code generated by flakesmith; DO NOT EDIT.

package flakedemo

import "testing"

func TestFlakeWitness(t *testing.T) {
	t.Run("polluter/TestSeedDB", TestSeedDB)
	t.Run("victim/TestCheckout", TestCheckout)
}
```

### 6.2. Package clause — the silent trap

A package can have two kinds of test files: `package foo` (inner) and `package foo_test` (outer). **These are two separate packages.** A test function of `foo` cannot be called from `foo_test` — `undefined: TestPolluter` (verified).

Rule:

1. Find the package clause of the file that defines the victim (`facts.TestClause`).
2. Is the clause the same for all polluters? If different → `ErrUnsupportedTest`, message: *"TestCheckout (package foo_test) cannot be called in the same test binary as TestSeedDB (package foo); the two live in different test packages."*
3. If the same, the harness is generated with that clause.

Also checked: is there a `TestMain` (if so, it runs too since the harness is in the same package — desired behavior, but stated in the report), does the name `TestFlakeWitness` already exist (if so, `TestFlakeWitness_2`).

### 6.3. File name and lifecycle

```go
func (h *Harness) Install(pkg string, steps []domain.Step, fn string) (string, error) {
	src := render(witnessTmpl, map[string]any{"Package": pkg, "Func": fn, "Steps": steps})
	formatted, err := format.Source([]byte(src))
	if err != nil {
		// If the code we generate doesn't compile, that's our bug, not the user's.
		return "", fmt.Errorf("%w: generated witness syntax is invalid: %v", err)
	}
	h.path = filepath.Join(h.dir, domain.WitnessFilePrefix+h.suffix+"_test.go")
	return h.path, os.WriteFile(h.path, formatted, 0o644)
}
```

- `format.Source` runs **before** writing: a broken harness never reaches disk.
- `zz_` prefix: files prefixed with `_`/`.` are skipped by the go tool (verified).
- `Remove()` via `defer` on every run; `errors.Is(err, fs.ErrNotExist)` is swallowed.
- **Signal safety:** `signal.NotifyContext(SIGINT, SIGTERM)` + `defer h.Remove()`; additionally, at startup `cleanupStale(dir)` deletes old `zz_flakesmith_witness_*` files (leftovers after a crash/kill).
- The user's worktree is never permanently polluted: the permanent artifact is written **only** by the `compile` command, on the user's explicit request.

---

## 7. Search: four probes, by cost order

Cheapest hypothesis first. If a probe returns `ErrNotReproduced`, move on to the next.

```go
type Probe interface {
	Name() string
	// Probe returns either a verified Witness or ErrNotReproduced.
	Probe(ctx context.Context, tgt Target, r gotest.Runner) (*domain.Witness, error)
}

type Target struct {
	Test   domain.TestID
	PkgDir string
	Facts  facts.PkgFacts
}
```

| Order | Probe | Cost | What it does | If found |
|---|---|---|---|---|
| 1 | `repeat` | **1 run** | `-count=10`; anything that passes the first round then fails? | `KindRepeat` |
| 2 | `order` | ~10–40 runs | passes alone, fails with all → shuffle sweep → ddmin | `KindOrder` |
| 3 | `parallel` | ~6 runs | passes with `-parallel=1`, fails with `-parallel=2,4,8` | `KindParallel` |
| 4 | `env` | ~4 runs/var | flip each variable read by `os.Getenv` to a sentinel | `KindEnv` |

### 7.1. `repeat` — the one-command flake detector

Verified: with `-count=5`, an error like "state survived into run 2" shows up in a single run. On `-count=1` it passes clean. This is the cheapest and most frequently productive of all hypotheses — hence first in line.

```go
func (p Repeat) Probe(ctx context.Context, tgt Target, r gotest.Runner) (*domain.Witness, error) {
	base, err := r.Run(ctx, domain.RunSpec{
		Package: tgt.PkgDir, RunRegex: "^" + tgt.Test.Name + "$", Count: 1,
	})
	if err != nil { return nil, err }
	if base.Outcomes[tgt.Test.Name].Status == domain.StatusFail {
		return nil, ErrNotReproduced // fails on its own too: not a flake
	}
	rep, err := r.Run(ctx, domain.RunSpec{
		Package: tgt.PkgDir, RunRegex: "^" + tgt.Test.Name + "$", Count: p.Count, // 10
	})
	if err != nil { return nil, err }
	if rep.Outcomes[tgt.Test.Name].Status != domain.StatusFail {
		return nil, ErrNotReproduced
	}
	return &domain.Witness{Kind: domain.KindRepeat, Victim: tgt.Test, Runs: 2}, nil
}
```

No minimization needed: one test, one hypothesis. The evidence reads directly ("it failed on round 2").

### 7.2. `order` — order dependency

```
1) victim alone, -parallel=1   →  if it fails: not a flake, stop
2) all tests, -parallel=1      →  if it fails: the observed order is in hand, go to 4
3) shuffle seeds [1..N]        →  take the order of the first failing seed, go to 4
4) shrink the tests BEFORE the victim with ddmin
5) re-verify the result 3 times
```

```go
func (p Order) Probe(ctx context.Context, tgt Target, r gotest.Runner) (*domain.Witness, error) {
	// 1) victim alone
	alone, err := r.Run(ctx, spec(tgt, `^`+tgt.Test.Name+`$`, 1))
	if err != nil { return nil, err }
	if alone.Outcomes[tgt.Test.Name].Status == domain.StatusFail {
		return nil, ErrNotReproduced
	}

	// 2) everything, source order, single thread
	full, err := r.Run(ctx, spec(tgt, "", 1)) // RunRegex="" → all tests
	if err != nil { return nil, err }
	if full.Outcomes[tgt.Test.Name].Status == domain.StatusFail {
		return p.minimize(ctx, tgt, r, full.Order)
	}

	// 3) shuffle sweep
	for _, seed := range p.Seeds {
		res, err := r.Run(ctx, domain.RunSpec{
			Package: tgt.PkgDir, Count: 1, Parallel: 1, ShuffleSeed: seed,
		})
		if err != nil { return nil, err }
		if res.Outcomes[tgt.Test.Name].Status == domain.StatusFail {
			return p.minimize(ctx, tgt, r, res.Order)
		}
	}
	return nil, ErrNotReproduced
}
```

### 7.3. ddmin + verification

Classic ddmin, but **a non-deterministic predicate produces false minimality**. That's why the result is re-verified N times; if it cannot be verified, `ErrNotReproducible`.

```go
type Predicate func(ctx context.Context, subset []domain.TestID) (bool, error)

// Minimize finds the smallest set satisfying the predicate "victim fails when the
// subset runs first", then re-verifies the result verify times.
func Minimize(ctx context.Context, cand []domain.TestID, pred Predicate, verify int) ([]domain.TestID, error) {
	min, err := ddmin(ctx, cand, pred)
	if err != nil { return nil, err }
	for i := 0; i < verify; i++ {
		ok, err := pred(ctx, min)
		if err != nil { return nil, err }
		if !ok { return nil, ErrNotReproducible }
	}
	return min, nil
}

// ddmin: split-try, then try the complement (classic Zeller algorithm).
func ddmin(ctx context.Context, cand []domain.TestID, pred Predicate) ([]domain.TestID, error) {
	n := 2
	for len(cand) >= 2 {
		chunks := split(cand, n)
		reduced := false

		for _, c := range chunks {             // is a single chunk enough?
			ok, err := pred(ctx, c)
			if err != nil { return nil, err }
			if ok { cand, n, reduced = c, 2, true; break }
		}
		if reduced { continue }

		for i := range chunks {                // drop this chunk, keep the rest
			comp := complement(chunks, i)
			if len(comp) == 0 { continue }
			ok, err := pred(ctx, comp)
			if err != nil { return nil, err }
			if ok { cand, reduced = comp, true; n = max(n-1, 2); break }
		}
		if reduced { continue }

		if n >= len(cand) { break }
		n = min(n*2, len(cand))
	}
	return cand, nil
}
```

### 7.4. Predicate: harness + runner

The search and the artifact use the same generator — the only difference is whether the file is permanent.

```go
func (p Order) predicate(tgt Target, r gotest.Runner) Predicate {
	return func(ctx context.Context, subset []domain.TestID) (bool, error) {
		steps := make([]domain.Step, 0, len(subset)+1)
		for _, t := range subset {
			steps = append(steps, domain.Step{Label: "polluter/" + t.Name, Test: t})
		}
		steps = append(steps, domain.Step{Label: "victim/" + tgt.Test.Name, Test: tgt.Test})

		h := harness.New(tgt.PkgDir)
		if _, err := h.Install(p.clause, steps, p.fn); err != nil { return false, err }
		defer h.Remove()

		res, err := r.Run(ctx, domain.RunSpec{
			Package: tgt.PkgDir,
			RunRegex: "^" + p.fn + "$", // only the witness we generated
			Count:   1, Parallel: 1, Race: p.Race,
		})
		if err != nil { return false, err }
		if res.BuildFailed { return false, fmt.Errorf("%w: %s", ErrBuildFailed, res.BuildOutput) }
		return res.Outcomes[tgt.Test.Name].Status == domain.StatusFail, nil
	}
}
```

Verified detail: **the victim still runs even if the polluter itself fails** — when a subtest fails, `t.Run` lets the parent continue. So ddmin gets the right answer even on polluters that blow up.

### 7.5. `parallel` and `env` (post-MVP but designing now)

`parallel`: baseline passes with `-parallel=1`, fails with `-parallel=2,4,8` → hypothesis confirmed. Minimization is over a **set, not an order**: which tests can be removed from the set while it still fails. `t.Cleanup` ordering and port collisions are the typical causes in this class.

`env`: for each variable in the `facts.EnvReads` list, run with `KEY=<synthetic>` and compare against the baseline. The value is never written into the capsule — only the **key name** and "effective/not effective".

---

## 8. Evidence collection

Evidence is a list of **observations**, not a "why" claim. Every field is optional; whatever is filled, the report shows it.

| Field | Source | Cost | When valuable |
|---|---|---|---|
| `EnvDiff` | `os.Environ()` diff of two runs (values masked) | 0 | Environment-caused flake |
| `GlobalsWritten` | AST: which package vars does the test function assign | 0 | Internal pollution (39% in real-world scans) |
| `GlobalsRead` | AST: which package vars does the victim read | 0 | Intersection = strong signal |
| `FilesTouched` | file paths in `-v` output + `t.TempDir()` patterns | 0 | External pollution |
| `PortsUsed` | `127.0.0.1:\d+` in `-v` output | 0 | Parallel port collision |
| `RaceOutput` | `-race` output (summarized) | 1 run | Real data race |
| `GoroutineTop` | first 3 lines of the stack at timeout/panic | 0 | Deadlock |

**Intersection rule:** if `GlobalsWritten ∩ GlobalsRead` is not empty, the report says "suspected cause: shared package variable `checkout.seeded` (written by TestSeedDB, read by TestCheckout)". This is **the single highest-value line** — because it tells the user where to look, without claiming a "root cause".

In the MVP, the AST walk is enough (`go/ast`); SSA (`golang.org/x/tools/go/ssa`) is optional in Phase 3, behind a build tag so as not to break K1.

---

## 9. Capsule and permanent artifact

### 9.1. Identity: content-addressed

Same commit + same victim + same witness → same ID. This is idempotency: a re-run in CI does not produce a new file.

```
id = sha256(schema | module | commit | victim.pkg | victim | kind
            | sort(polluters) | parallel | race | sort(envKeys))[:12]
```

Time, machine, and run count do not enter the identity: the same defect writes the
same file on every run — it becomes "one capsule per bug", not "one capsule per run".
`count` is deliberately excluded too — the same victim, the same polluter, the same
class is the same bug; the number of repetitions it was caught in does not change the identity.

```json
{
  "schema": "flakesmith/v1",
  "id": "fs_a3f19c02b7d4",
  "createdAt": "2026-11-01T00:42:11+03:00",
  "repo": {"module": "example.com/svc", "commit": "a1b2c3d", "dirty": false},
  "tool": {"version": "0.3.0", "go": "go1.26.2", "os": "darwin", "arch": "arm64"},
  "target": {"package": "example.com/svc/internal/checkout",
             "relDir": "./internal/checkout", "victim": "TestCheckout"},
  "kind": "order",
  "witness": {
    "kind": "order",
    "victim": {"package": "example.com/svc/internal/checkout", "name": "TestCheckout"},
    "polluters": [{"package": "example.com/svc/internal/checkout", "name": "TestSeedDB"}],
    "steps": [{"label": "polluter/TestSeedDB"},
              {"label": "victim/TestCheckout"}],
    "flags": {"parallel": 1},
    "seed": 1788950163029644000,
    "runs": 3
  },
  "observation": {"runs": 3, "failures": 3, "deterministic": true, "failureRate": 1},
  "evidence": {
    "globalsWritten": {"TestSeedDB": ["seeded"]},
    "globalsRead": {"TestCheckout": ["seeded"]},
    "race": null
  },
  "repro": {
    "command": "go test ./internal/checkout -tags flakesmith -run '^TestFlakeRegression_TestCheckout$' -count=1 -parallel=1",
    "generatedTest": "checkout_flakesmith_test.go"
  },
  "status": "confirmed"
}
```

The `steps[].test` field is **not written** to JSON (`json:"-"`): the name is already
inside `label`, and keeping it in two places would bloat the capsule. In a step read
back from a capsule, `Test` stays empty — code generation must use `Step.Func()`, not `Test.Name`.
`evidence` drops empty fields with `omitempty`; `race` is only filled in a `--race`
verification. `note` (if present) carries the one-sentence warning that `status`
does not say: "budget exceeded: 1/1 runs", "not reproduced under a race run".

`status`: `confirmed` | `partial` (budget exhausted) | `probabilistic` (external service; determinism not guaranteed).

### 9.2. Permanent test

The template is separate **per class**: reproducing the four classes requires different
code; one generic body used to stay green in three classes (the tool said "it will be
red", the test passed).

| Class | Body | Requirement to reproduce |
|---|---|---|
| `order` | `t.Run` chain (polluter → victim) | order |
| `parallel` | each step `t.Run` + `t.Parallel()` if `WrapParallel` | `-parallel=N`, slots = number of steps |
| `repeat` | the same victim `Flags.Count` times, in one process | `-count=N` |
| `env` | `t.Skip` when the variable is missing, then the victim | the variable itself (its value is not stored) |

```go
//go:build flakesmith

// Code generated by flakesmith; DO NOT EDIT.
//
// Capsule: fs_a3f19c02b7d4
// Class: order · Polluter: TestSeedDB
// Victim: TestCheckout
//
// Reproduce:
//   go test ./internal/checkout -tags flakesmith -run '^TestFlakeRegression_TestCheckout$' -count=1 -parallel=1

package checkout_test

import "testing"

func TestFlakeRegression_TestCheckout(t *testing.T) {
	t.Run("polluter/TestSeedDB", TestSeedDB)
	t.Run("victim/TestCheckout", TestCheckout)
}
```

The "Reproduce" line in the header is not written by hand: it comes from
`compile.CommandLine`, i.e. the class, `-race`, `-parallel`, `-count` and `-tags` truth
lives in one place. If the witness was found with `-race`, it also enters the command and
`VerifySpec`; otherwise, on a race-only flake the artifact stays green and `--verify`
wrongly says "the artifact does not reproduce".

`compile --verify` really runs the generated file **once**
(`RunSpec{Tags: [flakesmith], Race: the witness's, Parallel: width}`). If it is not
red, exit code 3: the claim "the artifact does not reproduce" is measured, not
a guess.

The price of the build tag: `go test ./...` does not see this. In exchange, the default suite is neither slowed down nor turned red. If the user removes the tag, it runs on every run — both options are written in the README.

**Important:** the test will be **red** right now (the flake is still there). The artifact's job is to catch the flake, not to fix it. After the fix it turns green and guards the regression permanently. That's why when `compile` says "the test will be RED right now", `--verify` proves it.

---

## 10. Budget, cancellation, error handling

```go
type Budget struct {
	MaxRuns int           // default 1000 (policy.DefaultMaxRuns)
	MaxWall time.Duration // default 5m
	runs    int
	started time.Time
}

func New(maxRuns int, maxWall time.Duration) *Budget {
	return &Budget{MaxRuns: maxRuns, MaxWall: maxWall, started: time.Now()}
}

// Spend is called at the START of every run. The time check happens at run
// start; the last run may overshoot by its own Timeout — this overshoot is bounded
// and deliberate (cutting a run in half produces a wrong negative).
func (b *Budget) Spend(n int) error {
	if b.runs+n > b.MaxRuns {
		return fmt.Errorf("%w: %d/%d runs", ErrBudgetExceeded, b.runs+n, b.MaxRuns)
	}
	if elapsed := time.Since(b.started); elapsed > b.MaxWall {
		return fmt.Errorf("%w: %s / %s", ErrBudgetExceeded, elapsed.Round(time.Second), b.MaxWall)
	}
	b.runs += n
	return nil
}
```

Every `Runner.Run` call does one `Spend(1)`, via a `Runner` wrapper:

```go
type budgeted struct {
	inner gotest.Runner
	b     *policy.Budget
}

func (r budgeted) Run(ctx context.Context, s domain.RunSpec) (*domain.RunResult, error) {
	if err := r.b.Spend(1); err != nil { return nil, err }
	return r.inner.Run(ctx, s)
}
```

No panic when the budget is exceeded: a `status: partial` capsule is written and **which steps were tried** is reported. A partial result is worth more than an empty one.

### 10.1. Typed errors → exit codes

| Condition | Exit | Message (to the user) |
|---|---|---|
| `hunt`: no witness (clean scan) | `0` | `"flake not found."` + `tried:` hypothesis list |
| `hunt`/`repro`: witness found, no `--fail-on-flake` | `0` | report + capsule; the gate stays open |
| `ErrFlakeFound` (`--fail-on-flake`, outside baseline) | `1` | "flake found" |
| `ErrUsage` | `2` | "usage: flakesmith repro <package-dir> [TestName]" |
| `ErrNotReproduced` | `3` | `repro`: requested test could not be reproduced within the budget; `replay`: witness did not fail N/N; `compile --verify`: generated test stayed green |
| `ErrBudgetExceeded` | `4` | "budget exceeded" + note: "raise the budget: --budget N --timeout 10m" (partial capsule written) |
| `ErrUnsupportedTest` | `5` | "unsupported test form: <reason>" (TestMain/subtest/different test package) |
| `ErrBuildFailed` | `6` | "package failed to build: <reason>" |
| internal error | `10` | "internal error: <id> (use --debug for detail)" |
| `context.Canceled` / SIGINT | `130` | "cancelled (SIGINT); unfinished witness files deleted" (128+SIGINT) |

The distinguishing rule: `hunt` asks "I looked around, there's nothing" — even a
found flake exits 0 while the gate is closed. `repro` asks about a single suspect;
if no answer comes, the request has failed (3). With `--baseline`, known identities do
not turn `hunt` red; only **new** flakes produce 1 with `--fail-on-flake`.

`--format json` always produces **a single document**; it is not line-delimited JSON:

```json
{"schema": "flakesmith/v1", "flakes": [ … capsule … ], "runs": 42, "maxRuns": 1000}
```

On a clean run: `"flakes": []` — not empty output, but the sentence "scanned and clean".

Without `--debug`, no stack trace is shown. A raw panic never leaks to the user (K5).

### 10.2. Concurrency

- Packages are scanned **sequentially** today; cross-package parallelism (`--jobs`,
  default `min(NumCPU, 4)`) is on hold as a pending task — we don't
  pretend it's implemented.
- **Two runs in the same package are never parallel** — they'd share the same port/temp file/DB and produce false positives against each other. A per-package `sync.Mutex` semaphore and a directory lock (`policy/dirlock`) also prevent a second flakesmith in the same directory.
- The width inside the `probe/parallel` class is given via `-parallel=N`; that is the witness's own load, not the concurrency of the package scan.

---

## 11. Testing strategy: developing without real flakes

The problem: producing real flakes in our own tests is not deterministic — our own tool would go flaky in CI. The solution is **two layers**:

### 11.1. FakeRunner — logic layer (fast, deterministic)

```go
type FakeRunner struct {
	Discoveries map[string][]domain.TestID
	// Script maps a spec to a deterministic result: the "fake universe".
	Script func(domain.RunSpec) (*domain.RunResult, error)
	Runs   []domain.RunSpec // records what was called (for assertions)
}

func (f *FakeRunner) Run(ctx context.Context, s domain.RunSpec) (*domain.RunResult, error) {
	f.Runs = append(f.Runs, s)
	return f.Script(s)
}
```

Example fake universe: `TestVictim` fails only when `RunRegex` covers all tests → the whole order probe + ddmin + harness flow is tested without a real `go test`. **This is the highest-leverage testing decision in the project:** probe/ddmin/classification logic is verified in milliseconds, 100% deterministically.

### 11.2. Fixture modules — integration (real `go test`)

`testdata/fixtures/*`, each with its own `go.mod`; they contain real flake types:

| Fixture | Content | Expected class |
|---|---|---|
| `order-basic` | package var; polluter writes, victim reads | `KindOrder`, polluter = TestSeedDB |
| `parallel-race` | `t.Parallel()` + shared counter | `KindParallel` |
| `env-dependent` | assertion branching on `os.Getenv("APP_ENV")` | `KindEnv`, key = APP_ENV |
| `file-leak` | polluter writing to a fixed path instead of `t.TempDir()` | `KindOrder` + `FilesTouched` |
| `repeat-accumulate` | global counter, fails on round 2 | `KindRepeat` |
| `clause-mismatch` | mixed `package foo` + `package foo_test` | `ErrUnsupportedTest` |

### 11.3. Golden files

The generated test file is compared byte-by-byte (`testdata/golden/*.go`). Code-generator regressions are caught here. Refreshed with the `-update` flag.

### 11.4. Our own tests must not be flaky

- Integration tests are skipped with `testing.Short()`; a separate CI job.
- No test depends on real time/ports/network.
- `go test -count=10 ./...` runs on every PR in CI (using our own tool on ourselves — dogfooding).

---

## 12. Task order and why this order

**Vertical slice first, horizontal layers later.** The first five tasks build an end-to-end pipeline with the `repeat` probe (discover → run → capsule → report). That way every later probe plugs into working infrastructure. Writing layer by layer (all types first, then the whole runner, then all probes) gives you the "nothing works" feeling in week 3.

### Task 1 — Module skeleton and domain types

**To do:** `go mod init github.com/emrelab/flakesmith`; write the §4 types into `internal/domain`; `names.go` constants; `errors.go` typed errors + `ExitCode(err) int`; `LICENSE`, `README` skeleton, `CI` workflow.
**Acceptance criteria:** `go build ./...` and `go vet ./...` clean; the `domain` package imports no internal package; the `ExitCode` table verified with a table test.
**Files:** `go.mod`, `internal/domain/*.go`, `internal/domain/domain_test.go`, `.github/workflows/ci.yml`.

### Task 2 — Runner: Discover + Run

**To do:** `Exec` runner (§5.1–5.2); `-json` parser; `normalize.go`; `*exec.ExitError` → normal-result distinction; build-failure detection; timeout.
**Acceptance criteria:** on `testdata/fixtures/order-basic`, `Discover` returns 3 test names in source order; via `Run`, a failing test yields `Outcomes[...].Status == StatusFail` and `err == nil`; `-count=1` is always in the arguments; a fixture with a build error yields `BuildFailed=true`.
**Files:** `internal/gotest/{runner,discover,json,normalize}.go` + tests.

### Task 3 — PkgFacts: AST scanner

**To do:** scan the package directory with `go/parser`; package clause map (test → file → clause); package-level `var` names; the package var each test function writes/reads; `os.Getenv` keys; presence of `TestMain`; subtest (`t.Run`) usage; `time.Now`/`rand`/`net` usage.
**Acceptance criteria:** in `order-basic`, `GlobalsWritten["TestSeedDB"] == ["seeded"]` and `GlobalsRead["TestCheckout"] == ["seeded"]`; in `clause-mismatch`, two different clauses are reported; the fixture with subtests has `Subtests=true`.
**Files:** `internal/facts/{scan,facts}.go` + `testdata/fixtures/*`.

### Task 4 — Harness: render + install/remove

**To do:** template (§6.1); `format.Source` pre-validation; `zz_flakesmith_witness_<suffix>_test.go` writing; `Remove()`; `cleanupStale()`; `SIGINT` capture; refusal on clause mismatch; function-name collision resolution.
**Acceptance criteria:** in `order-basic`, the generated harness really takes the victim down (5/5); `clause-mismatch` yields `ErrUnsupportedTest`; after `Ctrl-C`, no `zz_flakesmith_witness_*` remains in the directory; the golden file matches exactly.
**Files:** `internal/harness/*.go`, `testdata/golden/witness_basic_test.go`.

### Task 5 — `repeat` probe + capsule + text report (first end-to-end)

**To do:** `Repeat` probe (§7.1); `capsule.New/Write`; content-addressed ID; `report/text.go`; `cli.Run` skeleton + `hunt` command (repeat only).
**Acceptance criteria:** the output of `flakesmith hunt ./testdata/fixtures/repeat-accumulate` says "repeat flake: TestAccumulating, failed on round 2" and writes `.flakesmith/fs_*.json`; the same command produces **the same ID** the second time; `go build` still has zero external dependencies.
**Files:** `internal/probe/{probe,repeat,registry}.go`, `internal/capsule/*.go`, `internal/report/text.go`, `internal/cli/{cli,hunt}.go`, `cmd/flakesmith/main.go`.

> **At this point the product works.** The remaining tasks add new probes and outputs to the same pipeline — the architecture no longer changes.

### Task 6 — Budget, cancellation, error paths

**To do:** `policy.Budget` + `budgeted` wrapper (§10); `SIGINT` → `context` cancellation; `partial` capsule; wiring the exit-code table into `cli`.
**Acceptance criteria:** a run with `--budget 1` exits with code `4` and writes a `status: partial` capsule; after a `timeout` signal the harness is deleted.
**Files:** `internal/policy/*.go`, `internal/cli/cli.go`.

### Task 7 — ddmin + minimization verification

**To do:** `ddmin` and `Minimize` from §7.3; `FakeRunner` and table tests (synthetic universes: 1 polluter among 16 candidates, 2 polluters, no polluter).
**Acceptance criteria:** finds the single polluter among 16 candidates in ≤ 20 predicate calls; when the predicate lies 10% of the time, `Minimize` returns `ErrNotReproducible` (never silently returns a wrong result); coverage test: the found set is truly minimal (no element can be removed).
**Files:** `internal/search/{ddmin,minimize}.go` + tests.

### Task 8 — `order` probe

**To do:** the §7.2 flow; shuffle seed list; `predicate` (§7.4); evidence: global intersection.
**Acceptance criteria:** in `order-basic`, polluter=TestSeedDB is found; in `file-leak`, `FilesTouched` comes back filled; correct results also on fixtures where the polluter itself fails (the verified `t.Run` behavior); `clause-mismatch` yields `ErrUnsupportedTest`.
**Files:** `internal/probe/order.go`, `internal/evidence/static.go`.

### Task 9 — `compile`: permanent artifact

**To do:** build-tagged generation (§9.2); the `compile` command; `git diff` check (new files only); `--print` (stdout, without writing to disk).
**Acceptance criteria:** the generated file passes `go vet -tags flakesmith ./...` clean; `go test -tags flakesmith -run '^TestFlakeRegression_'` reproduces the flake; `go test ./...` (without tags) is unaffected; compiling the same capsule twice leaves the file unchanged.
**Files:** `internal/compile/*.go`, `internal/cli/compile.go`, `testdata/golden/persistent_*.go`.

### Task 10 — CLI completion + markdown/JSON output + `explain`/`replay`

**To do:** the `repro`, `replay`, `explain`, `init` commands; `--format text|json|markdown`; PR comment generator; `--fail-on-flake`.
**Acceptance criteria:** every command supports `--help`; `explain` output is produced from the capsule (no re-run); the `markdown` output renders as a table on GitHub; `cli.Run` is a pure function (stdout/stderr injected) and table-tested.
**Files:** `internal/cli/*.go`, `internal/report/{json,markdown}.go`.

### Task 11 — `parallel` and `env` probes

**To do:** §7.5; set-based minimization; env matrix; `--jobs` package parallelism + per-package lock.
**Acceptance criteria:** `parallel-race` is classified as `KindParallel`; `env-dependent` finds the right key and does **not** write the value into the capsule (redaction test).
**Files:** `internal/probe/{parallel,env}.go`, `internal/policy/redact.go`.

### Task 12 — GitHub Action, documentation, real-repo trial

**To do:** `action.yml`; `README` + 3 example capsules; `docs/`; smoke on 5 real open-source Go repos (against their known flaky issues); `--budget`/output tuning.
**Acceptance criteria:** the Action produces an artifact on the sample repo; at least 2 of the 5 real repos yield a reproducible witness; the README's "first 5 minutes" flow works when followed from scratch.
**Files:** `action.yml`, `README.md`, `docs/*.md`.

### 12.1. Realistic schedule (part-time, 15–20 hours/week)

| Task | Duration | Cumulative |
|---|---|---|
| 1–2 | 1.5 weeks | 1.5 |
| 3–4 | 1.5 weeks | 3 |
| **5 (first end-to-end)** | 1 week | **4** |
| 6–7 | 1 week | 5 |
| 8 | 1.5 weeks | 6.5 |
| 9 | 1 week | 7.5 |
| 10 | 1 week | 8.5 |
| 11 | 1.5 weeks | 10 |
| 12 | 1 week | **11 weeks** |

After task 5 the product is usable; 6–12 make it reliable and distributable. For a full-time developer: ~5–6 weeks.

---

## 13. Risks and open questions

| # | Risk | Impact | Mitigation |
|---|---|---|---|
| R1 | `-shuffle` seed space is huge; an order not found in 40 seeds | False negative | Default 40 seeds; raise with `--seeds`; the report says "not found in this range" |
| R2 | Tests use an external service/DB → reproduction impossible | `ErrNotReproduced` | `status: probabilistic`; warning to the user: "this package uses external dependencies" |
| R3 | Package contains 200+ tests; one ddmin sweep takes 40s | Budget not enough | Pre-filter with `-run`; `-failfast`; `--seeds 10` on expensive packages |
| R4 | On Windows, `exec`, path, and port behavior differ | Platform bug | MVP: darwin/linux guaranteed; windows best-effort, smoke in CI |
| R5 | `TestMain` resets global state; harness ineffective | Not findable | `facts.HasTestMain` → report note; the hypothesis is still tried |
| R6 | The user's `GOFLAGS` injects `-count` or `-mod=vendor` | Silent wrong result | `GOFLAGS` printed to the debug log; `-count` always passed explicitly (last wins) |
| R7 | `t.Parallel()` subtest order is unpredictable | Wrong classification | Order hypothesis only under `-parallel=1`; parallel is a separate class |
| R8 | The tool produces flakes against itself (harness leftovers) | Trust loss | `cleanupStale` + signal capture + `-count=10` dogfooding in CI |

---

## 14. What we will NOT do (scope fence)

The Red Team analysis (K1) makes this the MVP's survival condition:

- ❌ **Dashboard / web UI** — none. Output is stdout + JSON + markdown.
- ❌ **Server / SaaS / account** — none. Local files + CI artifacts.
- ❌ **LLM / automatic fix** — none. It shows evidence; it does not patch code.
- ❌ **Go toolchain patch / runtime fork / WASM** — none (ReplayGo's territory).
- ❌ **DB/HTTP replay** — none. External dependency → `probabilistic` label.
- ❌ **Subtest (`t.Run` inner) support** — outside the MVP; explicitly refused with `ErrUnsupportedTest`.
- ❌ **Cross-package order** — none. Same test binary required.
- ❌ **Test impact analysis / quarantine / flake trend** — Phase 5.

This fence is the written answer to "one more feature" requests: every item is a competitor's territory or a scope beyond a single developer.

---

## 15. One-page summary

| Question | Answer |
|---|---|
| What? | A CLI that drives `go test` and shrinks order/parallelism/environment flakes into a **permanent regression test** |
| How many packages? | 12 internal packages, all under `internal/`; external dependencies: **zero** |
| Most critical type? | `domain.RunSpec` / `RunResult` / `Witness` — single source of truth |
| Most critical interface? | `gotest.Runner` — the foundation of all testability |
| Most critical file? | `internal/harness` — both the search and the artifact generator |
| How is order controlled? | Temporary `zz_flakesmith_witness_*_test.go` + `t.Run` chain (`-run` cannot give order) |
| First working version? | End of task 5 (~4 weeks): `hunt` + repeat probe + capsule |
| MVP? | End of task 9 (~7.5 weeks): four probes + compile |
| What it doesn't do? | Dashboard, SaaS, LLM, runtime patch, DB replay |
