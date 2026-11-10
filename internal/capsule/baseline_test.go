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

func TestLegacyBaselineRequiresExplicitRegeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte(`{"schema":"flakesmith/baseline/v1","ids":["fs_old"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := capsule.LoadBaseline(path)
	if !errors.Is(err, domain.ErrUsage) || !errors.Is(err, capsule.ErrLegacyBaseline) || !strings.Contains(err.Error(), "--write-baseline") {
		t.Fatalf("migration error = %v", err)
	}
}

func TestFingerprintSeparatesDefectsFromRunParameters(t *testing.T) {
	w := domain.Witness{Kind: domain.KindOrder, Victim: domain.TestID{Package: "example/p", Name: "TestVictim"}, Polluters: []domain.TestID{{Name: "TestA"}, {Name: "TestB"}}}
	want := capsule.Fingerprint(w, "example")
	changed := w
	changed.Flags = domain.Flags{Parallel: 4, Race: true, Count: 50}
	changed.Func = "TestFlakeWitness_2"
	changed.Runs = 9
	changed.Seed = 42
	changed.Polluters = []domain.TestID{{Name: "TestB"}, {Name: "TestA"}}
	if got := capsule.Fingerprint(changed, "example"); got != want {
		t.Fatalf("run metadata changed fingerprint: %s != %s", got, want)
	}
	for _, mutation := range []func(*domain.Witness){
		func(w *domain.Witness) { w.Victim.Name = "TestOther" },
		func(w *domain.Witness) { w.Kind = domain.KindParallel },
		func(w *domain.Witness) { w.Polluters = []domain.TestID{{Name: "TestOther"}} },
		func(w *domain.Witness) { w.EnvDiff = []string{"MODE"} },
		func(w *domain.Witness) { w.CrossBinary = true },
	} {
		different := w
		mutation(&different)
		if capsule.Fingerprint(different, "example") == want {
			t.Fatal("different defect matched baseline")
		}
	}
	if capsule.ID(w, "example", "commit1") == capsule.ID(w, "example", "commit2") {
		t.Fatal("capsule provenance must remain commit-specific")
	}
}

func TestBaselineIsNotListedAsCapsule(t *testing.T) {
	dir := t.TempDir()
	if err := capsule.WriteBaseline(filepath.Join(dir, capsule.BaselineFileName), []string{"fd_known"}); err != nil {
		t.Fatal(err)
	}
	if ids, err := capsule.NewStore(dir).List(); err != nil || len(ids) != 0 {
		t.Fatalf("baseline listed as evidence: %v %v", ids, err)
	}
}
