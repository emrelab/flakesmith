// Package facts extracts the static facts of a test package.
//
// go/ast is enough for the MVP: package clauses, package-level vars, which
// package vars each test function writes and reads, os.Getenv keys, TestMain,
// and subtest usage. SSA is a Phase 3 option behind a build tag so the zero
// dependency rule survives.
//
// Why it matters: the intersection of "polluter writes X" and "victim reads X"
// is the single most valuable line in the report — it tells the user where to
// look without claiming a root cause.
//
// Contract: docs/ARCHITECTURE.md §6.2, §8. Task 3.
package facts
