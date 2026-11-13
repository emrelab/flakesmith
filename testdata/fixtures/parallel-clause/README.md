# fixture: parallel-clause

**Expected class:** `ErrUnsupportedTest` (exit 5) — the shape is `KindParallel`,
but no witness can be built for it.

Two clauses, one test binary: `TestInnerAlpha` lives in `parallelclause`,
`TestOuterVictim` in `parallelclause_test`. Both call `t.Parallel()`, so under
`-parallel>=2` they are inside the shared resource at the same time and the
victim fails. That is a real concurrency flake, not an ordering one — the two
clauses are two *visibility* boundaries inside ONE binary, not two processes.

Why it is unsupported: the generated witness is written into the victim's clause
and calls test functions by name, so `TestInnerAlpha` is `undefined` from
`parallelclause_test`. Before the fix the candidate list was built by name only,
the cross-clause name reached the template, and the probe died with
`domain.ErrInternal: witness did not compile` — which `ddmin` propagated instead
of a clean refusal. Now cross-clause candidates are split off, never enter the
witness, and the probe reports this file's expected verdict.

**Expected:** `go-flake-hunter hunt .` → `ErrUnsupportedTest`, exit code **5**, and
the message names both tests and both packages.

Contract (`go test`, no hunter involved):

- `-parallel=1` → PASS: tests are serialised, nothing overlaps.
- `-parallel>=2` → FAIL on `TestOuterVictim` only; `TestInnerAlpha` is the
  innocent polluter and passes.
- `-run '^TestOuterVictim$'` → PASS: the victim alone is not broken.

The overlap is a channel, not a polled timer (same design as `parallel-race`,
issue #12): the second arriver closes it and the first wakes in microseconds.
`OverlapWait` is a fallback for a run that genuinely has no sibling; the
`-test.run`/`-test.parallel` checks in `CanOverlap` keep the tool's solo passes
free of it.

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own:
`cd testdata/fixtures/parallel-clause && go build ./...`

The probe-level paths are covered without a real `go test` in
`internal/probe/parallel_test.go` (`TestParallelReportsCrossClauseInsteadOfInternal`,
`TestParallelReportsCrossClauseWhenSameClauseFailsToReproduce`,
`TestParallelIgnoresCrossClauseWhenSameClauseExplains`).
