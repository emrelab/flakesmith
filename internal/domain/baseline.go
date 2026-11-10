package domain

// BaselineSchemaVersion is the schema of the baseline file, separate from the capsule
// schema: a baseline is not a capsule but a declaration that "these ids are known".
// It increases on breaking changes; if an old file cannot be read the tool says so.
const BaselineSchemaVersion = "flakesmith/baseline/v2"

// Baseline holds the known flake identities.
//
// IDs contains commit-independent defect fingerprints, not capsule IDs.
// Capsule IDs still identify evidence from a particular commit.
type Baseline struct {
	Schema string   `json:"schema"`
	IDs    []string `json:"ids"`
}

// Contains reports whether this id is in the baseline.
//
// An empty baseline considers nothing "known": if the file is broken or
// was never written, gate behavior must not change.
func (b Baseline) Contains(id string) bool {
	for _, known := range b.IDs {
		if known == id {
			return true
		}
	}
	return false
}
