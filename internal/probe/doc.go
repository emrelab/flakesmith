// Package probe tries the four flake hypotheses in cost order.
//
// repeat → order → parallel → env. Each probe either returns a verified
// Witness or ErrNotReproduced so the registry can move on.
//
// The order is the product: a repeat flake costs ONE run, an order flake tens.
// Trying the cheap hypothesis first is what keeps the default budget usable.
//
// Contract: docs/ARCHITECTURE.md §7. Tasks 5, 8, 11.
package probe
