package orderbasic

import "testing"

func TestPolluter(t *testing.T) {
	seeded = 42 // pollutes the shared state, never cleans it
}

func TestVictim(t *testing.T) {
	if seeded != 0 {
		t.Fatalf("shared state polluted: seeded=%d", seeded)
	}
}

func TestUnaffected(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("this test must never fail under any condition")
	}
}
