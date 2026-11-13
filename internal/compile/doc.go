// Package compile turns a capsule into a permanent regression test.
//
// This is the artifact that makes go-flake-hunter different from a flake detector:
// the failure becomes a file in the repository, reviewable in a PR and visible
// to `git blame`, instead of another dashboard row.
//
// The generated test carries -tags goflakehunter so the default `go test ./...`
// neither slows down nor turns red. Removing the tag makes it run every time.
//
// Contract: docs/ARCHITECTURE.md §9.2. Task 9.
package compile
