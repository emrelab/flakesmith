package harness_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/harness"
)

// Generator regression: the output must stay byte-for-byte stable. When the
// test file changes, the golden must be updated deliberately too.
func TestRenderMatchesGolden(t *testing.T) {
	steps := []domain.Step{
		{Label: domain.PolluterLabel("TestPolluter"), Test: domain.TestID{Name: "TestPolluter"}},
		{Label: domain.VictimLabel("TestVictim"), Test: domain.TestID{Name: "TestVictim"}},
	}
	got, err := harness.Render("orderbasic", "TestFlakeWitness", steps)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "golden", "witness_order_basic_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("cannot read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("generator output does not match golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
