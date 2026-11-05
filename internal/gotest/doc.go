// Package gotest runs `go test` and parses its JSON event stream.
//
// The whole tree depends on the Runner interface; this is the only package
// that calls exec. Contract: docs/ARCHITECTURE.md §5. Task 2.
package gotest
