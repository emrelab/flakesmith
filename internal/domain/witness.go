package domain

import "strings"

// Kind is the flake class: the output of the four probes in docs/ARCHITECTURE.md §7.
type Kind string

const (
	KindRepeat   Kind = "repeat"   // fails when repeated in the same process
	KindOrder    Kind = "order"    // fails when another test runs first
	KindParallel Kind = "parallel" // fails only when run in parallel
	KindEnv      Kind = "env"      // depends on an environment variable
)

// Valid confirms the class is a known value (capsule schema check).
func (k Kind) Valid() bool {
	switch k {
	case KindRepeat, KindOrder, KindParallel, KindEnv:
		return true
	}
	return false
}

// Flags are the go test flags needed for reproduction.
type Flags struct {
	Parallel int  `json:"parallel,omitempty"`
	Race     bool `json:"race,omitempty"`
	// Count is the -count value a repeat witness was found with ("--repeat N").
	// Without it a capsule found with -count=50 replays with the verification
	// count (2), goes green, and the tool reports "flake fixed" — a false
	// diagnosis that costs the user trust in every later capsule.
	Count int `json:"count,omitempty"`
}

// Step is a single t.Run call inside the harness.
//
// The Test field is not written to JSON: its name is already inside Label and
// keeping it in two places would bloat the capsule. But a Step read back from
// a capsule has an empty Test — code generation must use Func(), not Test.Name.
type Step struct {
	Label string `json:"label"` // "polluter/TestSeedDB"
	Test  TestID `json:"-"`
	// WrapParallel marks a step whose test does NOT call t.Parallel() itself.
	// The parallel witness has to add the call for it, otherwise the steps stay
	// serial and the witness proves nothing. A test that already calls
	// t.Parallel() must be left alone: testing.T panics on the second call
	// ("t.Parallel called multiple times").
	WrapParallel bool `json:"wrapParallel,omitempty"`
}

// Func returns the test function name this step calls.
//
// Prefers the in-memory TestID and falls back to parsing the label, so a step
// reconstructed from a capsule renders the same code as one built in memory.
func (s Step) Func() string {
	if s.Test.Name != "" {
		return s.Test.Name
	}
	if i := strings.LastIndex(s.Label, "/"); i >= 0 && i+1 < len(s.Label) {
		return s.Label[i+1:]
	}
	return s.Label
}

// Witness is the minimal witness found: the smallest set of preceding tests that makes the victim fail.
type Witness struct {
	Kind      Kind     `json:"kind"`
	Victim    TestID   `json:"victim"`
	Polluters []TestID `json:"polluters,omitempty"`
	Cleaners  []TestID `json:"cleaners,omitempty"`
	Steps     []Step   `json:"steps"`
	Flags     Flags    `json:"flags"`
	Seed      int64    `json:"seed,omitempty"`    // -shuffle seed for KindOrder
	EnvDiff   []string `json:"envDiff,omitempty"` // key names only
	Runs      int      `json:"runs"`              // verified over how many runs
}

// Minimal reports whether this is a verified minimal witness (at least one polluter and at least 2 runs).
func (w *Witness) Minimal() bool { return len(w.Polluters) > 0 && w.Runs >= 2 }
