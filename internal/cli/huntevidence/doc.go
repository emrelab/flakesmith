// Package clihuntevidence holds the tests for what hunt reports beyond the bare
// finding: the race label, the SARIF document, the cross-binary (two-process)
// path, and the parallel-race fixture contract that every concurrency claim is
// calibrated against.
//
// Test-only package: the split exists so `go test ./...` can schedule the
// integration suites in parallel (see internal/clitest).
package clihuntevidence
