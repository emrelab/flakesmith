package gotest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
)

// synthStream builds a synthetic `go test -json` stream for n tests: pre-stream
// noise, a package start, then per test a run, one output line and a terminal
// event. Every other test fails so the worst-status merge has work to do. Pure
// in-memory bytes — a benchmark must never spawn a `go test` subprocess.
func synthStream(n int) []byte {
	var sb strings.Builder
	sb.WriteString("go: building flakesmith/bench...\n")
	sb.WriteString("{\"Action\":\"start\",\"Package\":\"bench/x\",\"Time\":\"2026-11-02T01:00:00Z\"}\n")
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("TestGen%03d", i)
		status := "pass"
		if i%2 == 1 {
			status = "fail"
		}
		fmt.Fprintf(&sb, "{\"Action\":\"run\",\"Package\":\"bench/x\",\"Test\":%q}\n", name)
		fmt.Fprintf(&sb, "{\"Action\":\"output\",\"Package\":\"bench/x\",\"Test\":%q,\"Output\":\"=== RUN   %s\\n\"}\n", name, name)
		fmt.Fprintf(&sb, "{\"Action\":%q,\"Package\":\"bench/x\",\"Test\":%q,\"Elapsed\":0.012}\n", status, name)
	}
	fmt.Fprintf(&sb, "{\"Action\":\"fail\",\"Package\":\"bench/x\",\"Elapsed\":%d.4}\n", n)
	return []byte(sb.String())
}

// BenchmarkJSONStream measures the -json fold over realistic stream sizes:
// a small package (25), a medium one (100) and a heavy suite (400). The stream
// is built once per size, outside the timed region; the loop measures the fold.
func BenchmarkJSONStream(b *testing.B) {
	for _, n := range []int{25, 100, 400} {
		stream := synthStream(n)
		b.Run(fmt.Sprintf("tests=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			spec := domain.RunSpec{Package: "./bench", Count: 1}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res, err := parseEvents(stream, spec, 1, 0, false)
				if err != nil {
					b.Fatal(err)
				}
				if len(res.Outcomes) != n {
					b.Fatalf("outcomes = %d, want %d", len(res.Outcomes), n)
				}
			}
		})
	}
}
