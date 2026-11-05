# fixture: clause-mismatch

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own: `cd testdata/fixtures/clause-mismatch && go build ./...`

There are two test packages: `clausemismatch` (inner) and `clausemismatch_test`
(outer). These are **two separate test binaries**; within one process they
cannot call each other's functions. But `go test .` runs the inner binary
first, then the outer one; the file the inner binary leaves on disk really
does fail the victim in the outer binary.

That is why no witness can be installed: the witness form calls test functions
in sequence inside a single binary and cannot express cross-process pollution.

**Expected:** `flakesmith hunt .` → `ErrUnsupportedTest`, exit code **5**, and
the message names both tests and their packages. Saying "no flake" would be a
lie.

- Alone: `go test -run '^TestOuterVictim$' .` is clean (TestMain deletes the file).
- Whole package: `go test .` is red (the inner binary leaves the file, the outer binary sees it).

The two probe-level paths are verified with table tests in
`internal/probe/order_test.go` (no same-class candidates / same-class candidates
do not explain).
