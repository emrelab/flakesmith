package capsule

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/emrelab/flakesmith/internal/domain"
)

// ErrLegacyBaseline requires an explicit rescan; v1 hashes cannot be reversed.
var ErrLegacyBaseline = errors.New("baseline v1 uses commit-specific capsule IDs; regenerate with hunt --write-baseline")

// BaselineFileName is the name of the known-flake list inside the capsule directory.
const BaselineFileName = "baseline.json"

// DefaultBaselinePath is <dir>/.flakesmith/baseline.json.
//
// The path is relative to the invoked directory, not the capsule directory:
// `hunt ./...` scans many packages but the baseline is one per repository. One
// baseline per package would mean two different "known" sets in the same repo.
func DefaultBaselinePath(dir string) string {
	return filepath.Join(dir, domain.CapsuleDir, BaselineFileName)
}

// LoadBaseline reads and validates a baseline file.
//
// A broken file or one with a mismatched schema version is not silently
// treated as an "empty baseline": the gate would then go red on every flake
// and the user would have to hunt for the reason in the file. The error is a
// usage error (exit 2) because the problem is in the file named by the argument.
func LoadBaseline(path string) (domain.Baseline, error) {
	var b domain.Baseline
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return b, fmt.Errorf("%w: baseline not found: %s", domain.ErrUsage, path)
		}
		return b, fmt.Errorf("%w: could not read baseline: %s: %v", domain.ErrUsage, path, err)
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return b, fmt.Errorf("%w: baseline is corrupt (%s): %v", domain.ErrUsage, path, err)
	}
	if b.Schema == "flakesmith/baseline/v1" {
		return b, fmt.Errorf("%w: %w", domain.ErrUsage, ErrLegacyBaseline)
	}
	if b.Schema != domain.BaselineSchemaVersion {
		return b, fmt.Errorf("%w: baseline schema version not supported: %q (expected %q)",
			domain.ErrUsage, b.Schema, domain.BaselineSchemaVersion)
	}
	return b, nil
}

// WriteBaseline persists the known ids, deduplicated and sorted.
//
// Order and uniqueness are guaranteed at write time: otherwise the same set
// produces two different files and the baseline's commit changes on every
// run — unacceptable noise for this project's own suite.
func WriteBaseline(path string, ids []string) error {
	sorted := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)

	data, err := json.MarshalIndent(domain.Baseline{
		Schema: domain.BaselineSchemaVersion,
		IDs:    sorted,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	tmp, err := os.CreateTemp(dir, ".baseline-*.json")
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	// A half-written file would be a "corrupt baseline" on the next --baseline call;
	// same temp+rename rule as Store.Write.
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	return nil
}
