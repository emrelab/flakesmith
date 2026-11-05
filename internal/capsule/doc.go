// Package capsule writes and reads the persistent record of a witness.
//
// The id is content-addressed, so running the same hunt twice on the same
// commit produces the same file instead of a new one every time. That
// idempotency is what makes capsules usable as CI artifacts.
//
// Contract: docs/ARCHITECTURE.md §9.1. Task 5.
package capsule
