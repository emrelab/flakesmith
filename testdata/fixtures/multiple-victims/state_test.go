package multiplevictims

import "testing"

var dirtyA, dirtyB bool

func TestVictimA(t *testing.T) {
	if dirtyA {
		dirtyA = false
		t.Fatal("A was polluted")
	}
}

func TestVictimB(t *testing.T) {
	if dirtyB {
		dirtyB = false
		t.Fatal("B was polluted")
	}
}

func TestPolluter(t *testing.T) { dirtyA, dirtyB = true, true }
