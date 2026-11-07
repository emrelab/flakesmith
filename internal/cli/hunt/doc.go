// Package clihunt holds the hunt engine's integration tests: end-to-end runs
// against the fixtures, the JSON document contract, and the flags a user types.
// Each test spawns real `go test` subprocesses through the CLI.
//
// Test-only package: the split exists so `go test ./...` can schedule the
// integration suites in parallel (see internal/clitest).
package clihunt
