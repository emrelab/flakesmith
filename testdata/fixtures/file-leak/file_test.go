package fileleak

import (
	"os"
	"path/filepath"
	"testing"
)

// Expected: KindOrder + Evidence.FilesTouched populated.
// Polluter writes to a fixed path (no t.TempDir); victim expects the file to be absent.
func fixturePath() string { return filepath.Join(os.TempDir(), "flakesmith-fixture-leak.txt") }

func TestPolluterWritesFile(t *testing.T) {
	if err := os.WriteFile(fixturePath(), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVictimExpectsNoFile(t *testing.T) {
	if _, err := os.Stat(fixturePath()); err == nil {
		t.Fatalf("file already exists: %s", fixturePath())
	}
}

func TestMain(m *testing.M) {
	_ = os.Remove(fixturePath()) // start clean at startup
	os.Exit(m.Run())
}
