package envdependent

import (
	"os"
	"testing"
)

// Expected: KindEnv, key = FIXTURE_MODE.
// Whatever the value, the test only fails on values other than "ci".
func TestDependsOnEnv(t *testing.T) {
	if os.Getenv("FIXTURE_MODE") == "ci" {
		t.Fatalf("behavior changed in CI mode")
	}
}
