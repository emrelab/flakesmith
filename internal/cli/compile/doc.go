// Package clicompile holds the compile tests: the three output surfaces
// (--print, --dry-run, write), the package-target rule, and --verify, which
// proves the generated regression test actually goes red.
//
// Test-only package: the split exists so `go test ./...` can schedule the
// integration suites in parallel (see internal/clitest).
package clicompile
