// Package clisurface holds the CLI's own surface tests: flags, help text,
// usage errors, init and explain. They spawn no subprocesses and finish in
// milliseconds; keeping them apart from the hunt tests means a failure here
// names the surface, not the engine.
//
// Test-only package: the split exists so `go test ./...` can schedule the
// integration suites in parallel (see internal/clitest).
package clisurface
