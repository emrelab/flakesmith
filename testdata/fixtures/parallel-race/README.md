# fixture: parallel-race

**Expected class:** `KindParallel`

`-parallel=1` → PASS · `-parallel>=2` → FAIL, and always on `TestParallelBeta`
(the victim). `TestParallelAlpha` is the innocent polluter: it shares the
resource and passes. See the design notes in `par_test.go`.

Design note: a single test function never runs in parallel with its own copy
(per the Go `testing` docs), so "the incrementing test + `t.Parallel()`" is not
enough. The fixture uses a barrier that requires two **different** tests to be
active at once; classification therefore depends on real concurrency, not on
ordering.

The barrier is a channel, not a timer: the second arriver closes it and the
first wakes immediately, so the failing path costs microseconds and the witness
verification is deterministic under load. The 30s deadline is a fallback for
"no sibling was selected in this run" and is skipped when `-test.run` cannot
match the sibling — the tool's solo/verification passes (`-run ^TestParallelBeta$`)
never pay it.

Earlier revisions used a fixed 40ms window, then a 5ms poll for up to 2s. Both
were load-sensitive: a squeeze under load looked like "no parallel flake", the
hunter honestly refused the witness, and the fixture itself flaked (issue #12).

The contract is asserted by `TestParallelRaceFixtureContract`
(`internal/cli/huntevidence/evidence_fixture_test.go`): serial passes, overlap fails on
the victim only, the victim alone passes, the fallback deadline is never paid
when a sibling was selected, and a solo run costs no more than the serial
baseline. That last clause is the one that fails if a fixed wait comes back.

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own:
`cd testdata/fixtures/parallel-race && go build ./...`
