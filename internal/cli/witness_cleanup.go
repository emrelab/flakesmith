package cli

import (
	"os"
	"path/filepath"

	"github.com/emrelab/flakesmith/internal/domain"
)

// cleanupWitnesses removes witness files left by a killed run.
//
// A stray zz_flakesmith_witness_* file makes every later `go build` in that
// package fail with test errors that look like the user's fault, so this runs
// before anything else touches the directory.
func cleanupWitnesses(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		if e.IsDir() || !domain.IsWitnessFileName(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.Remove(p); err != nil {
			continue
		}
		removed = append(removed, p)
	}
	return removed, nil
}
