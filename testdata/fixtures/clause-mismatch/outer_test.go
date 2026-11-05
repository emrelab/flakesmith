package clausemismatch_test

import (
	"os"
	"path/filepath"
	"testing"

	"fixture/clausemismatch"
)

// Expected: ErrUnsupportedTest (exit 5).
//
// This test lives in package clausemismatch_test, while TestInnerPolluter is in
// package clausemismatch. They are two separate test binaries: they cannot call
// each other in the same process (undefined: TestInnerPolluter), but `go test .`
// runs the inner binary first and then the outer binary — the file the inner
// binary leaves on disk really does bring down the victim in the outer binary.
// This is a class our witness form (calling test functions in sequence inside a
// single binary) cannot express; saying "no flake" would be a lie.
func fixturePath() string { return filepath.Join(os.TempDir(), "flakesmith-fixture-clause.txt") }

func TestOuterVictim(t *testing.T) {
	clausemismatch.SetShared(0)
	if _, err := os.Stat(fixturePath()); err == nil {
		t.Fatalf("test in inner package left a file: %s", fixturePath())
	}
}
