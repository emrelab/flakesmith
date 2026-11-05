// Package policy: budget, cancellation and secret redaction.
//
// Every run spends one entitlement from the budget; env values are never written to artifacts.
// Contract: docs/ARCHITECTURE.md §10, §7.5.
package policy
