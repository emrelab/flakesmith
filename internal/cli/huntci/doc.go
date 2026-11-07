// Package clihuntci holds the tests for hunt's CI-facing subset: the baseline
// file (a known flake must not break the gate) and --changed (only the packages
// a diff touched). They are separate from the core hunt tests because they
// drive a real git repository and a working tree, not just a fixture.
//
// Test-only package: the split exists so `go test ./...` can schedule the
// integration suites in parallel (see internal/clitest).
package clihuntci
