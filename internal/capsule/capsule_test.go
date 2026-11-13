package capsule_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emrelab/go-flake-hunter/internal/capsule"
	"github.com/emrelab/go-flake-hunter/internal/domain"
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
	if len(a) != len("gfh_")+12 {
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
	store := capsule.NewStore(filepath.Join(dir, ".go-flake-hunter"))

	c := domain.Capsule{
		Schema:    domain.SchemaVersion,
		ID:        "gfh_test123456",
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
	if err := os.WriteFile(filepath.Join(dir, "gfh_old.json"),
		[]byte(`{"schema":"go-flake-hunter/v0","id":"gfh_old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("gfh_old"); err == nil {
		t.Fatal("the old schema must be rejected")
	}
}

// A capsule written before the rename keeps its id and stays readable: the
// rename changed the schema string, not the document, so refusing it would
// throw away evidence that is still true.
func TestStoreReadsPreRenameSchema(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)
	body := `{"schema":"flakesmith/v1","id":"gfh_legacy","kind":"order",` +
		`"status":"confirmed","witness":{"victim":{"package":"p","name":"TestV"}}}`
	if err := os.WriteFile(filepath.Join(dir, "gfh_legacy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read("gfh_legacy")
	if err != nil {
		t.Fatalf("a pre-rename capsule must still load: %v", err)
	}
	if got.ID != "gfh_legacy" || got.Schema != domain.LegacySchema {
		t.Errorf("read back id=%q schema=%q", got.ID, got.Schema)
	}
}

func TestStoreListAndMissing(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)

	ids, err := store.List()
	if err != nil || len(ids) != 0 {
		t.Fatalf("empty dir: ids=%v err=%v", ids, err)
	}
	if _, err := store.Read("gfh_missing"); err == nil {
		t.Fatal("a missing capsule must return an error")
	}
}

// A half-written file would blow up the next read: writes go through rename.
func TestStoreWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)
	if _, err := store.Write(domain.Capsule{Schema: domain.SchemaVersion, ID: "gfh_a"}); err != nil {
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

// An unknown flake class is a usage error, not an internal one.
//
// The class selects the regression body and the report sentence; a hand-edited
// or truncated capsule used to sail through Read and only explode inside the
// generator as "internal error: unknown flake class", which tells the user
// nothing about the file they are holding. Partial capsules are exempt: they
// carry no class by construction and are never compiled.
func TestReadRejectsUnknownKind(t *testing.T) {
	dir := t.TempDir()
	store := capsule.NewStore(dir)
	bad := domain.Capsule{
		Schema: domain.SchemaVersion,
		ID:     "gfh_bad",
		Kind:   domain.Kind("wobble"),
		Target: domain.Target{Package: "example.com/m", Victim: "TestX"},
		Status: domain.StatusConfirmed,
	}
	if _, err := store.Write(bad); err != nil {
		t.Fatal(err)
	}
	_, err := store.Read("gfh_bad")
	if !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("want ErrUsage, got %v", err)
	}
	if !strings.Contains(err.Error(), "wobble") {
		t.Errorf("the message must name the bad class: %v", err)
	}

	partial := bad
	partial.ID = "gfh_partial"
	partial.Status = domain.StatusPartial
	if _, err := store.Write(partial); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("gfh_partial"); err != nil {
		t.Errorf("a partial capsule has no class by design: %v", err)
	}
}

func TestWitnessConfidenceAndSelectionIdentity(t *testing.T) {
	w := witness()
	confirmed := capsule.ID(w, "m", "commit")
	fingerprint := capsule.Fingerprint(w, "m")
	o := domain.Observe(3, 2)
	w.Verification = &o
	if capsule.ID(w, "m", "commit") == confirmed {
		t.Fatal("suspected capsule would overwrite confirmed capsule")
	}
	w.Verification = nil
	w.CrossClause = true
	w.Seed = 21
	if capsule.ID(w, "m", "commit") == confirmed {
		t.Fatal("top-level selection collides with generated witness")
	}
	if capsule.Fingerprint(w, "m") != fingerprint {
		t.Fatal("selection mechanics changed the known defect")
	}
	previous := capsule.ID(w, "m", "commit")
	w.Seed = 34
	if capsule.ID(w, "m", "commit") == previous {
		t.Fatal("reproduction seed missing from capsule identity")
	}
}
