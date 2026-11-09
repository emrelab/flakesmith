# fixture: synctest-bubble

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own: `cd testdata/fixtures/synctest-bubble && go build ./...`

**Expected fact:** `TestSynctest` marks exactly the tests that call
`synctest.Run`/`synctest.Wait`; the plain test stays unmarked. The fixture is
NOT flaky — a bubble's clock is virtual, so its verdict is deterministic by
construction. That is the point: a flagged test whose verdict still varies is
a real code bug, never "time/network/DB dependent" (issue #22).

Needs Go ≥ 1.25 (`testing/synctest` graduated to the stdlib there).
