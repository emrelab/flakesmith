// Package clireplay holds the replay and repro tests: both take a recorded
// artifact — a capsule, or a failed `go test -json` log — and re-execute it,
// which is why they share a package.
//
// Test-only package: the split exists so `go test ./...` can schedule the
// integration suites in parallel (see internal/clitest).
package clireplay
