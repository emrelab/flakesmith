package domain

import "time"

// SchemaVersion is the version of the capsule JSON schema.
// It is increased on breaking changes; old capsules remain readable.
const SchemaVersion = "flakesmith/v1"

// CapsuleStatus is the confidence level of a witness.
type CapsuleStatus string

const (
	// StatusConfirmed reproduced deterministically.
	StatusConfirmed CapsuleStatus = "confirmed"
	// StatusPartial the budget ran out; what was tried is reported.
	StatusPartial CapsuleStatus = "partial"
	// StatusProbabilistic there is an external dependency; determinism is not guaranteed.
	StatusProbabilistic CapsuleStatus = "probabilistic"
)

// Repo is the code state the run was performed against.
type Repo struct {
	Module string `json:"module"`
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

// Tool is the tool's identity, for report reproducibility.
type Tool struct {
	Version string `json:"version"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Target is the test under investigation.
type Target struct {
	// Package is the import path (stable identity, survives a different cwd).
	Package string `json:"package"`
	// RelDir is the directory as the user typed it, so the reported
	// reproduce command is runnable from where they invoked flakesmith.
	RelDir string `json:"relDir,omitempty"`
	Victim string `json:"victim"`
}

// Observation is how often it failed across how many runs.
type Observation struct {
	Runs          int     `json:"runs"`
	Failures      int     `json:"failures"`
	Deterministic bool    `json:"deterministic"`
	Rate          float64 `json:"failureRate"`
}

// Evidence records observations; it is not a "root cause".
type Evidence struct {
	GlobalsWritten map[string][]string `json:"globalsWritten,omitempty"`
	GlobalsRead    map[string][]string `json:"globalsRead,omitempty"`
	EnvDiff        []string            `json:"envDiff,omitempty"`
	FilesTouched   []string            `json:"filesTouched,omitempty"`
	Ports          []string            `json:"ports,omitempty"`
	Race           *string             `json:"race,omitempty"`
	GoroutineTop   []string            `json:"goroutineTop,omitempty"`
}

// Repro is how to reproduce the failure.
type Repro struct {
	Command       string `json:"command"`
	GeneratedTest string `json:"generatedTest,omitempty"`
}

// Capsule is the persistent artifact. The content-addressed ID makes it idempotent.
type Capsule struct {
	Schema      string        `json:"schema"`
	ID          string        `json:"id"`
	CreatedAt   time.Time     `json:"createdAt"`
	Repo        Repo          `json:"repo"`
	Tool        Tool          `json:"tool"`
	Target      Target        `json:"target"`
	Kind        Kind          `json:"kind"`
	Witness     Witness       `json:"witness"`
	Observation Observation   `json:"observation"`
	Evidence    Evidence      `json:"evidence"`
	Repro       Repro         `json:"repro"`
	Status      CapsuleStatus `json:"status"`
	// Note explains a non-confirmed status in one user-facing sentence
	// ("budget exceeded: 1/1 runs") or annotates a confirmed capsule whose
	// evidence is weaker than the status suggests ("not reproduced in the
	// race run"). Confirmed capsules with full evidence leave it empty.
	// Additive field: old readers ignore it, the schema version does not change.
	Note string `json:"note,omitempty"`
}
