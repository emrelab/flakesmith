package a

import (
	"os"
	"testing"
)

// TestBad is the cross-binary polluter (#6): it writes state to its OWN
// package directory. `go test` runs with cwd = package dir, and stage 2
// (package b, a separate process) reads exactly that file — the pollution
// survives the process boundary, which is the whole point of this class.
func TestBad(t *testing.T) {
	if err := os.WriteFile("pollution.txt", []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestNoop is innocent and must shrink out of the witness.
func TestNoop(t *testing.T) {}
