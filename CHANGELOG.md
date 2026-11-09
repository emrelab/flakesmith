# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `PkgFacts.TestSynctest` (issue #22): the facts scan now names tests that use
  the `testing/synctest` bubble API (`Test`/`Run`/`Omega`/`Wait`/`Go`, all
  spellings across the previews and the Go 1.26 graduation). A bubble's clock
  is virtual, so a flagged test whose verdict still varies is a real code bug —
  the "time/network/DB dependent" excuse cannot cover it. Naming only: the tool
  never synthesizes bubbles around someone else's test. Fixture:
  `synctest-bubble` (needs Go ≥ 1.26).
- `scripts/auto/land.sh`: the independent verdict for a direct push. Waits for
  the check runs on a `main` commit, calls green only when every run concluded
  `SUCCESS`/`NEUTRAL`, and on red reverts, pushes the revert and exits 1. Exit 2
  is inconclusive (no check run, or none concluded in time) and never reverts —
  undoing work that may have been fine is worse than a late human look.
- `scripts/auto/digest.sh` / `make auto-digest`: the human's half of the
  autonomous loop in one screen — open `auto/*` PRs with age, CI verdict,
  mergeability and diffstat; `main`'s own CI verdict and recent reverts;
  `auto:blocked` issues; and the cron's state with the two mismatches worth
  naming (paused with work queued, armed on an empty queue). Read-only:
  `gh pr list`, `gh issue list`, `gh api` and one `jq` read, no writes.
  `docs/AUTO.md` §11 says why a run cannot report this itself.
- `docs/CI-REPRO.md`: the CI glue for a red `go test` log — capture the `-json`
  log as an artifact, `repro --from-json` it, post one upserted PR comment with
  the witness. Comments only on exit 0: exit 3/4 means no witness, and silence is
  the honest answer. The workflow is a template (not wired into `.github/`), and
  the page says which half was verified locally.
- `scripts/test-cost.sh` / `make cost`: wall time, per-package time and the
  slowest tests. An operator tool for this exact tuning, not a gate — a
  wall-time assertion is the flakiness issue #12 removed.
- `TestDefaultPassesTheConfigThrough`: `probe.Default` owns the probe list, so
  the test reads `Repeat.Count` and `Order.Seeds` back off the registered probes
  — a transposed `Config` field would otherwise hunt with the wrong `-count` and
  no order assertion would notice.
- `TestHuntRecordsCapsuleProvenance`: the capsule's `repo.commit` and
  `repo.dirty` are asserted against a repository the test owns, in both the
  struct and the JSON a CI consumer reads.
- `TestScanRecordsGlobalsWrittenThroughEveryShape`: all five assignment shapes
  the AST scanner must see through (`x =`, `x.f =`, `x[i] =`, `*p =`, `(x) =`).

### Changed
- `auto:safe` no longer opens a PR: after `verify.sh --tier safe` it pushes
  `auto/issue-N:main` (explicit refspec, never forced) and `land.sh` takes the
  CI verdict, reverting on red. A PR paid for CI before touching `main`; this
  pays after, which is the whole trade — mechanical work is cheap to revert,
  and the review tier keeps the PR because a design decision is not.
  `docs/AUTO.md` §7 states both tiers, §8.4 the new stop condition.
- The CLI's integration suite is split into six test-only packages —
  `internal/cli/{hunt,huntci,huntevidence,compile,replay,surface}` — over a
  shared `internal/clitest` harness. `go test` runs tests inside a package
  serially but packages in parallel, and every CLI test spawns real `go test`
  subprocesses (a single `hunt` on `order-basic` spawns 33 `go` processes), so
  the single package sat on one core for its whole 185s. Same assertions, same
  count: the `-json` test-name set (subtests included) is 239 before and 239
  after, with no additions. Local wall time 185s → 50-55s; `-race` 184s → 56s.
- PR repeat sweep narrower and honest about it: `-count=3` now covers the unit
  and probe packages, and the integration layer runs once per OS per PR. A
  repeat exists to surface state that survives between runs in one process;
  there it multiplies subprocess spawns instead (18m35s on windows-latest at
  `-count=3`). No CI failure to date came from a repeat-only leak in that layer,
  and the nightly `-count=10` sweep on all three OSes still covers it.

### Fixed
- `release.yml`'s publish job ran `gh release create` with no checkout, so gh
  could not resolve the repository ("failed to run git: fatal: not a git
  repository") and the first v0.2.0 tag push died at the last step with all
  five platform builds green. The command now passes `--repo
  "$GITHUB_REPOSITORY"` explicitly — the job has no working tree by design.
- The README's GitHub Actions snippet pinned `emrelab/flakesmith@v0.2.0`, a tag
  that was never cut (`git tag` has only `v0.1.0`, `gh release list` is empty),
  so a copy-pasted workflow could not resolve the action. The example now pins
  `main` — the one ref that is both current and real — with a note to pin a `v*`
  release tag once one is published; `docs/ARCHITECTURE.md`'s status line and the
  install paragraph claimed the same nonexistent release and now say what is
  true. Issue #18.
- `docs/ARCHITECTURE.md` §14 no longer bans "cross-package order" outright: the
  `--polluter-pkg` two-stage engine ships, so the fence now excludes what is
  actually out of reach — in-memory state across two `go test` processes — and
  the note that two *clauses* of one directory link into one binary.
- The autonomous loop's review tier could feed itself: a run opened a PR and
  dropped `auto:wip`, which left the issue open, still `auto:review`, and the
  very next `next.sh` call picked it again — one design re-proposed up to
  `MAX_ISSUES` times per tick. The review tier now also adds `auto:pr`, and the
  picker excludes it; a human merges (or closes) the PR and clears the label to
  put the issue back in the queue. `docs/AUTO.md` §0/§2/§7 and
  `CONTRIBUTING.md` state the third resting state; `next.sh --selftest` covers
  both shapes.
- `scripts/auto/run.sh` claimed "there is no cron on purpose" while
  `docs/AUTO.md` §10 documents the scheduled drain; the comment now names the
  one-shot path and the cron job separately.
- `parallel-race` fixture no longer times out into a false "no overlap": the
  barrier is a channel closed by the second arriver (the first wakes in
  microseconds) instead of a 5ms poll of a counter for a fixed 2s. The 30s
  fallback deadline is skipped when the run's `-test.run` cannot select a
  sibling, so the tool's solo/verification passes never pay it. The victim is
  fixed by name (`TestParallelBeta`), so arrival order no longer decides which
  test fails — the witness verification that reported "2/3" on loaded windows
  (issue #12) is now deterministic. `hunt` on this fixture: 56s → 6s, every
  witness run 2.1s → instant.
- New `TestParallelRaceFixtureContract` asserts that instrument directly: the
  serial run passes, the overlapped run fails on the victim only, the victim
  alone passes, the fallback deadline is never paid when a sibling was
  selected, and a solo run costs no more than the serial baseline (the guard
  that fails on the old polling design: 2.27s vs 0.25s).
- CI timeouts back to measured values: the race step 20m → 15m and the
  `-count=3` sweep 40m → 30m. Both bumps existed only to absorb the
  `parallel-race` window; with it gone the windows job runs the race step in
  4m06s (was 9m20s) and the sweep in 13m02s (was 24m04s).
- Pre-commit gate: a commit touching `.github/workflows/` widens the gate to
  the full suite again. The pattern's `$` anchor made `^\.github/workflows/$`
  match only a directory name, which is never a staged file, so CI-only
  commits — exactly the ones the gate exists for — ran build+vet alone.
- `make cover` collapsed from 78% to 51% after the split: `go test
  -coverprofile` collects a package's coverage only from tests physically
  inside it, and the CLI's tests now live next door. `-coverpkg=./...` restores
  it (82.5%, identical to the pre-split number measured the same way).
- Two tests hunted the live `testdata` fixture instead of a copy
  (`TestHuntEndToEnd`, `TestHuntBudgetWritesPartialCapsule`), which made them
  depend on the surrounding working tree — their git state leaked into the
  capsule they produced. They now use a copy like every other test; the
  coverage that accident provided is replaced by the explicit provenance test
  above.

## [0.2.0] — 2026-11-06

### Changed
- `go.mod` floor lowered from 1.26 to 1.24 (measured: `t.Chdir` is the newest
  API in use); CI now proves the floor on a real 1.24 toolchain (`oldest` job).
- CI: per-PR repeat sweep is now `-count=3` on three OSes; the full
  `-count=10` sweep moved to a nightly workflow (`nightly.yml`).
- CI: new `self-hunt` job runs `flakesmith hunt` on flakesmith's own repo.
- The `parallel-race` fixture's fixed 40ms barrier replaced by an adaptive
  wait, so the fixture no longer loses its overlap window under load.
- GitHub Action: the JSON flake counter pins the capsule id prefix
  (`"id": "fs_`), so a future field named `id` cannot inflate the count.

### Added
- SECURITY.md, dependabot (github-actions), issue/PR templates, CHANGELOG.
- Release workflow (`release.yml`): a `v*` tag publishes CGO-free static
  binaries (linux/darwin amd64+arm64, windows amd64) with a `SHA256SUMS`
  file as a GitHub Release; `workflow_dispatch` builds the matrix without
  publishing.
- GitHub Action: at a tag ref the action now downloads the matching release
  binary, verifies its SHA-256 and its version stamp, and only then skips
  the build; on any miss (branch/SHA ref, missing asset, mismatch, no `gh`)
  it falls back to the unchanged source build. Auth uses the caller's
  `github.token`, overridable via the new `github-token` input.
- Verification contract (#8): `compile --verify` separates `verified`,
  `green-not-reproduced` and `skipped-missing-env`, records the verdict in
  the capsule's `verifyStatus`, and exits 3 unless the artifact actually
  reproduced the red — a skipped run can no longer read as a green
  verification in CI.
- Env detection: `os.Getenv`/`os.LookupEnv` reads in the package's
  non-test sources are collected as app-level evidence and probed after the
  test-local keys — a test that consumes an env var through application
  code is now reachable.
- `repro --from-json FILE|-` (#5): takes a failed `go test -json` log,
  folds subtests onto their parents, and reproduces the suspect list —
  CI's existing failure output is now a direct input.
- SARIF 2.1.0 (#4): `hunt --format sarif`, a `--sarif FILE` side file that
  pairs with any format, and victim file+line pinned from the AST scan into
  the capsule. The action gained an opt-in `upload-sarif` step (code
  scanning annotations never redden the job by themselves).
- Cross-binary order witness (#6): `hunt --polluter-pkg PKG` finds pollution
  that survives the process boundary (files, ports), records the two-stage
  `go test && go test` reproduction, replays it deterministically, and
  refuses compile-time inlining honestly. In-memory globals across binaries
  remain out of scope by physics, not by omission.
- Fuzz targets for the JSON stream parser and the capsule store, ddmin
  property tests (shrink preserves the predicate, stays a subset, is
  idempotent and 1-wise-minimal), and benchmarks for the parser, ddmin and
  the scanner at 25/100/400-test scale.
- `hunt/repro --short`: run the whole scan with `go test -short`. Operator
  choice on the runner, not a spec property — for repos whose integration
  tests honor `testing.Short()`, so a shuffle scan does not re-run them per
  seed. The PR self-hunt uses it; nightly added a full unshorted sweep.

### Fixed
- CI self-hunt: the budget guard was dead code — GitHub's default shell is
  `bash -e`, so a non-zero hunt exit killed the step before `rc` was read
  (surfaced as self-hunt @ d92e895). Guard rewritten (`|| rc=$?`) and the
  PR sweep now runs `--short`, with the unshorted sweep moved to nightly.

## [0.1.0] — 2026-11-04

### Added
- First public release: `hunt`, `repro`, `compile`, `replay`, `explain`,
  `init`; four probe classes (repeat, order, parallel, env); capsules;
  GitHub Action; fixtures for every flake class; zero external dependencies.

[Unreleased]: https://github.com/emrelab/flakesmith/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/emrelab/flakesmith/releases/tag/v0.2.0
[0.1.0]: https://github.com/emrelab/flakesmith/releases/tag/v0.1.0
