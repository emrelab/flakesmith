package clean

import "testing"

// Expected: no probe explains it → exit 3.
//
// The two tests are deliberately independent of each other: no shared global,
// no files, no environment reads, no t.Parallel. This is the reference point
// for the "clean package"; every other fixture shows one kind of deviation.
func TestSum(t *testing.T) {
	if 2+2 != 4 {
		t.Fatal("arithmetic is broken")
	}
}

func TestSplit(t *testing.T) {
	if len("a,b") != 3 {
		t.Fatal("unexpected string length")
	}
}
