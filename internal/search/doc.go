// Package search shrinks a candidate set to the smallest witness.
//
// Classic ddmin, plus a verification loop that the literature needs and the
// textbook version lacks: the predicate runs a real test binary, so it can
// answer differently twice for the same input. A minimizer that trusts a single
// "yes" would report a witness that reproduces 1 time in 10.
//
// Contract: docs/ARCHITECTURE.md §7.3. Task 7.
package search
