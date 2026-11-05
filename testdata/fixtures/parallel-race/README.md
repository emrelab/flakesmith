# fixture: parallel-race

**Expected class:** `KindParallel`

`-parallel=1` → PASS · `-parallel>=2` → FAIL (deterministic under load, see
the `overlapWait` note in `par_test.go`).

Design note: a single test function never runs in parallel with its own copy
(per the Go `testing` docs), so "the incrementing test + `t.Parallel()`" is
not enough. The fixture uses a barrier that requires two **different** tests
to be active at once; classification therefore depends on real concurrency,
not on ordering. The barrier waits adaptively (up to 500ms) for a late
sibling: the original fixed 40ms window lost the overlap on loaded CI runners
and made the fixture itself flaky — which, for a flake hunter's testdata, is
a bug of the highest order.

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own:
`cd testdata/fixtures/parallel-race && go build ./...`
