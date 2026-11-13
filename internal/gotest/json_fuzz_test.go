package gotest

import (
	"testing"
	"time"

	"github.com/emrelab/go-flake-hunter/internal/domain"
)

// FuzzParseEvents hammers the `go test -json` stream parser with arbitrary bytes.
//
// The contract lives in json.go and is deliberately forgiving: pre-stream noise
// is skipped, a corrupt record is skipped ("a single corrupt record must not
// sink the whole run"). Malformed input must therefore never panic. The only
// legitimate error is the line scanner giving up on a record longer than the
// 8 MiB buffer, and even then the fold must not half-succeed.
func FuzzParseEvents(f *testing.F) {
	seeds := []string{
		streamOnePassOneFail,
		streamPassThenFail,
		streamWithSubtests,
		// Pre-stream noise, exactly what `go test` prints before -json starts.
		"go: downloading github.com/example/dep\n{\"Action\":\"run\",\"Test\":\"TestX\"}\n{\"Action\":\"pass\",\"Test\":\"TestX\",\"Elapsed\":0.1}\n",
		// One corrupt record between two good ones: the run survives.
		"{\"Action\":\"run\",\"Test\":\"TestX\"}\nNOT JSON AT ALL\n{\"Action\":\"pass\",\"Test\":\"TestX\",\"Elapsed\":0.1}\n",
		// Right shape, wrong field types: Unmarshal errors, record skipped.
		`{"Action":"output","Test":"TestX","Output":{"not":"a string"}}`,
		`{"Action":"pass","Test":"TestX","Elapsed":"0.1"}`,
		// Extreme Elapsed values: the float->Duration cast must not panic.
		`{"Action":"pass","Test":"TestX","Elapsed":1e30}`,
		`{"Action":"fail","Test":"TestX","Elapsed":-1e30}`,
		// Truncated record (killed `go test`), lone brace, non-object JSON.
		`{"Action":"output","Test":"T`,
		"{",
		`[1,2,3]`,
		`"just a string"`,
		// Odd but legal strings in the identity fields.
		"{\"Action\":\"run\",\"Test\":\"Test\x00Weird/子\",\"Output\":null}",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		spec := domain.RunSpec{Package: "./fuzzpkg", Count: 1}
		res, err := parseEvents(data, spec, 0, time.Second, false)
		if err != nil {
			// Scanner failure (overlong record) must arrive without a result.
			if res != nil {
				t.Fatalf("error %v returned together with a result", err)
			}
			return
		}
		if res == nil {
			t.Fatal("nil result without error")
		}
		if res.Spec.Package != spec.Package {
			t.Errorf("spec not preserved: %+v", res.Spec)
		}
		// Invariants the fold promises regardless of input: top-level run
		// order is deduped, and no identity field can come back empty.
		seen := make(map[string]bool, len(res.Order))
		for _, name := range res.Order {
			if name == "" {
				t.Error("empty test name in Order")
			}
			if seen[name] {
				t.Errorf("duplicate in Order: %q", name)
			}
			seen[name] = true
		}
		for name, o := range res.Outcomes {
			if name == "" {
				t.Error("empty test name in Outcomes")
			}
			if o.Status > domain.StatusFail {
				t.Errorf("%s: status out of range: %d", name, o.Status)
			}
		}
	})
}
