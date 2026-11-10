package clausemismatch_test

import (
	"fixture/clausemismatch"
	"testing"
)

func TestOuterVictim(t *testing.T) {
	if clausemismatch.Shared() != 0 {
		t.Fatal("inner clause polluted shared memory")
	}
}
