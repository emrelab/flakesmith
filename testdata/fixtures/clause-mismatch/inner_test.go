package clausemismatch

import (
	"os"
	"path/filepath"
	"testing"
)

// fixturePath is out-of-process state: the only channel left between two test binaries.
// The `pkg` binary exits, the `pkg_test` binary starts after it; nothing shared
// remains in memory, but a file left on disk does.
func fixturePath() string { return filepath.Join(os.TempDir(), "flakesmith-fixture-clause.txt") }

func TestMain(m *testing.M) {
	// Each binary starts clean: pollution travels only within the same `go test .` run,
	// from the inner binary to the outer one.
	_ = os.Remove(fixturePath())
	os.Exit(m.Run())
}

func TestInnerPolluter(t *testing.T) {
	SetShared(1)
	if err := os.WriteFile(fixturePath(), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
}
