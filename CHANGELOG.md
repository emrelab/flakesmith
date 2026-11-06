# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project adheres to [Semantic Versioning](https://semver.org/).

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
- Env detection (#2): `os.Getenv`/`os.LookupEnv` reads in the package's
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

## [0.1.0] — 2026-11-04

### Added
- First public release: `hunt`, `repro`, `compile`, `replay`, `explain`,
  `init`; four probe classes (repeat, order, parallel, env); capsules;
  GitHub Action; fixtures for every flake class; zero external dependencies.

[Unreleased]: https://github.com/emrelab/flakesmith/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/emrelab/flakesmith/releases/tag/v0.2.0
[0.1.0]: https://github.com/emrelab/flakesmith/releases/tag/v0.1.0
