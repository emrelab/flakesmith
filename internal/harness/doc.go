// Package harness renders, installs and removes the temporary witness test file.
//
// This is the heart of the product: searching for a witness and producing the
// permanent regression test use the SAME generator — only the destination
// differs. That is why the report could claim "Go-native search + compiled
// test" rather than "another flake detector".
//
// A witness file is named zz_goflakehunter_witness_<suffix>_test.go because the go
// tool ignores files whose name starts with "_" or "." (verified 2026-11-01),
// so "zz_" is required, not cosmetic.
//
// Contract: docs/ARCHITECTURE.md §6. Tasks 4, 9.
package harness
