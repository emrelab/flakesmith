package facts_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/facts"
)

// A package-level var can be assigned through five shapes, and the scanner's
// job is to see through all of them — the write set is what tells a later hunt
// which globals a test can dirty.
//
// Until this test, only the plain `x = ...` shape was ever exercised, and only
// by accident: it was reached because a CLI test hunted a fixture that sits
// inside go-flake-hunter's own repository. The other four shapes were covered by
// nothing at all, which is exactly the kind of hole a coverage number hides
// (it read as covered, then dropped when that accident was removed).
func TestScanRecordsGlobalsWrittenThroughEveryShape(t *testing.T) {
	dir := t.TempDir()
	src := `package p

import "testing"

var (
	counter int
	box     struct{ n int }
	arr     [3]int
	ptr     *int
)

func TestWrites(t *testing.T) {
	counter = 1      // Ident
	box.n = 2        // SelectorExpr
	arr[0] = 3       // IndexExpr
	*ptr = 4         // StarExpr
	(counter) = 5    // ParenExpr
	local := 6       // not a global: must be ignored
	_ = local
}

func TestReadsOnly(t *testing.T) {
	_ = counter
}
`
	if err := os.WriteFile(filepath.Join(dir, "writes_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := facts.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := append([]string(nil), f.GlobalsWritten["TestWrites"]...)
	sort.Strings(got)
	want := []string{"arr", "box", "counter", "ptr"}
	if len(got) != len(want) {
		t.Fatalf("TestWrites globals = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TestWrites globals = %v, want %v", got, want)
		}
	}

	// A read is not a write: counting it would make every reader look like a
	// polluter and bury the real ones.
	if w := f.GlobalsWritten["TestReadsOnly"]; len(w) != 0 {
		t.Errorf("a test that only reads was reported as writing %v", w)
	}
	if r := f.GlobalsRead["TestReadsOnly"]; len(r) != 1 || r[0] != "counter" {
		t.Errorf("TestReadsOnly globals read = %v, want [counter]", r)
	}
}
