package search_test

import (
	"context"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/search"
)

// BenchmarkMinimize measures the ddmin search over a large failing set: 128
// candidates with a single polluter parked at the end, the shape that forces
// the most chunk/complement probes before convergence. The predicate is a pure
// name scan — in-memory only, no builds, no subprocesses — so the number
// reports the search algorithm's cost, and predcalls/op its oracle budget.
func BenchmarkMinimize(b *testing.B) {
	const n = 128
	ctx := context.Background()
	input := ids(n)
	pred := contains("Test127")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calls := 0
		counted := func(ctx context.Context, s []domain.TestID) (bool, error) {
			calls++
			return pred(ctx, s)
		}
		got, err := search.Minimize(ctx, input, counted, 1)
		if err != nil {
			b.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "Test127" {
			b.Fatalf("witness = %v, want [Test127]", got)
		}
		b.ReportMetric(float64(calls), "predcalls/op")
	}
}
