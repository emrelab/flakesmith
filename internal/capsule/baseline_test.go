package capsule_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/domain"
)

// The baseline is sorted and deduplicated when written to disk: otherwise the
// same set produces a different file on every run and the baseline commit
// becomes noise.
func TestWriteBaselineSortsAndDedupes(t *testing.T) {
	path := filepath.Join(t.TempDir(), domain.CapsuleDir, capsule.BaselineFileName)
	if err := capsule.WriteBaseline(path, []string{"fs_ccc", "fs_aaa", "fs_ccc", ""}); err != nil {
		t.Fatalf("could not write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), domain.BaselineSchemaVersion) {
		t.Errorf("schema version missing:\n%s", data)
	}
	b, err := capsule.LoadBaseline(path)
	if err != nil {
		t.Fatalf("could not read back: %v", err)
	}
	want := []string{"fs_aaa", "fs_ccc"}
	if len(b.IDs) != len(want) {
		t.Fatalf("ids = %v, want %v", b.IDs, want)
	}
	for i := range want {
		if b.IDs[i] != want[i] {
			t.Fatalf("ids = %v, want %v", b.IDs, want)
		}
	}
}

// The default path is a single file relative to the invoked directory — not per package.
func TestDefaultBaselinePath(t *testing.T) {
	got := capsule.DefaultBaselinePath("/repo")
	want := filepath.Join("/repo", domain.CapsuleDir, capsule.BaselineFileName)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A missing or corrupt baseline is not silently treated as "empty": the gate
// would then go red on every flake and the user could not see the reason.
func TestLoadBaselineRejectsBrokenFiles(t *testing.T) {
	dir := t.TempDir()

	if _, err := capsule.LoadBaseline(filepath.Join(dir, "missing.json")); !errors.Is(err, domain.ErrUsage) {
		t.Errorf("missing file: err = %v, want ErrUsage", err)
	}

	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := capsule.LoadBaseline(corrupt); !errors.Is(err, domain.ErrUsage) {
		t.Errorf("corrupt file: err = %v, want ErrUsage", err)
	}

	oldSchema := filepath.Join(dir, "old.json")
	if err := os.WriteFile(oldSchema, []byte(`{"schema":"flakesmith/baseline/v0","ids":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := capsule.LoadBaseline(oldSchema); !errors.Is(err, domain.ErrUsage) {
		t.Errorf("old schema: err = %v, want ErrUsage", err)
	}
}
