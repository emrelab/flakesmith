package domain

import "time"

// TestID identifies a test function together with its package.
// Name is always an exported function prefixed with "Test".
type TestID struct {
	Package string `json:"package"` // import path
	Name    string `json:"name"`    // "TestCheckout"
}

// String returns the full name in package.path form.
func (t TestID) String() string { return t.Package + "." + t.Name }

// IsZero reports whether the identity is empty.
func (t TestID) IsZero() bool { return t.Name == "" }

// Status is the outcome of a single test.
type Status uint8

const (
	StatusSkip Status = iota // never ran, or t.Skip
	StatusPass
	StatusFail
)

// String is for log and report output.
func (s Status) String() string {
	switch s {
	case StatusPass:
		return "pass"
	case StatusFail:
		return "fail"
	default:
		return "skip"
	}
}

// Outcome is a single test's result in a single run.
// Output is normalized (timestamp, tmp path, pointer stripped).
type Outcome struct {
	Status  Status
	Elapsed time.Duration
	Output  string
}
