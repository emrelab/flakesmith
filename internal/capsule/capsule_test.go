package capsule_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/domain"
)

func witness() domain.Witness {
	return domain.Witness{
		Kind:      domain.KindOrder,
		Victim:    domain.TestID{Package: "p", Name: "TestVictim"},
		Polluters: []domain.TestID{{Package: "p", Name: "TestPolluter"}},
		Flags:     domain.Flags{Parallel: 1},
		Runs:      5,
	}
}

// Same commit + same witness = same ID. CI must not produce a new file on every run.
func TestIDIsStable(t *testing.T) {
	a := capsule.ID(witness(), "m", "abc123")
	b := capsule.ID(witness(), "m", "abc123")
	if a != b {
		t.Fatalf("ID changed: %s != %s", a, b)
	}
	if len(a) != len("fs_")+12 {
		t.Errorf("ID format: %q", a)
	}
}

// Things that change must change the ID: a different witness is a different bug.
func TestIDChangesWithWitness(t *testing.T) {
	base := capsule.ID(witness(), "m", "abc123")

	w2 := witness()
	w2.Polluters = []domain.TestID{{Package: "p", Name: "TestOther"}}
	if capsule.ID(w2, "m", "abc123") == base {
		t.Error("changing the polluter must change the ID")
	}

	if capsule.ID(witness(), "m", "def456") == base {
		t.Error("changing the commit must change the ID")
	}

	w3 := witness()
	w3.Flags.Parallel = 4
	if capsule.ID(w3, "m", "abc123") == base {
		t.Error("changing a flag must change the ID")
	}

	// -race is part of the proof: a witness verified without race is not the same bug,
	// because it was confirmed without seeing what the race detector sees.
	w4 := witness()
	w4.Flags.Race = true
	if capsule.ID(w4, "m", "abc123") == base {
		t.Error("changing race must change the ID")
	}
}

// Polluter order must not affect the ID: same set, same bug.
func TestIDIgnoresPolluterOrder(t *testing.T) {
	w1 := witness()
	w1.Polluters = []domain.TestID{{Name: "TestA"}, {Name: "TestB"}}
	w2 := witness()
	w2.Polluters = []domain.TestID{{Name: "TestB"}, {Name: "TestA"}}
	if capsule.ID(w1, "m", "c") != capsule.ID(w2, "m", "c") {
		t.Error("polluter order must not affect the ID")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(filepath.Join(dir, ".flakesmith"))

	c := domain.Capsule{
		Schema:    domain.SchemaVersion,
		ID:        "fs_test123456",
		CreatedAt: time.Now().Truncate(time.Second),
		Kind:      domain.KindOrder,
		Witness:   witness(),
		Status:    domain.StatusConfirmed,
	}
	path, err := store.Write(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file was not written: %v", err)
	}

	got, err := store.Read(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != c.ID || got.Kind != c.Kind {
		t.Errorf("round-trip broken: %+v", got)
	}
	if len(got.Witness.Polluters) != 1 {
		t.Errorf("polluters lost: %+v", got.Witness)
	}
}

func TestStoreRejectsUnknownSchema(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)
	if err := os.WriteFile(filepath.Join(dir, "fs_old.json"),
		[]byte(`{"schema":"flakesmith/v0","id":"fs_old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("fs_old"); err == nil {
		t.Fatal("the old schema must be rejected")
	}
}

func TestStoreListAndMissing(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)

	ids, err := store.List()
	if err != nil || len(ids) != 0 {
		t.Fatalf("empty dir: ids=%v err=%v", ids, err)
	}
	if _, err := store.Read("fs_missing"); err == nil {
		t.Fatal("a missing capsule must return an error")
	}
}

// A half-written file would blow up the next read: writes go through rename.
func TestStoreWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)
	if _, err := store.Write(domain.Capsule{Schema: domain.SchemaVersion, ID: "fs_a"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || len(e.Name()) > 5 && e.Name()[0] == '.' {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
