# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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

## [0.1.0] — 2026-11-04

### Added
- First public release: `hunt`, `repro`, `compile`, `replay`, `explain`,
  `init`; four probe classes (repeat, order, parallel, env); capsules;
  GitHub Action; fixtures for every flake class; zero external dependencies.

[Unreleased]: https://github.com/emrelab/flakesmith/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/emrelab/flakesmith/releases/tag/v0.1.0
