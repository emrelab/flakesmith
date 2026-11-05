package facts

import "sort"

// PkgFacts is the static knowledge sheet for one test package directory.
type PkgFacts struct {
	// Dir is the package directory as given to Scan.
	Dir string
	// ImportPath is resolved from go.mod-relative position when available.
	ImportPath string

	// Clause maps a test function name to its file's package clause
	// ("flakedemo" or "flakedemo_test"). Two different clauses mean two
	// different test binaries: a harness cannot call across them.
	Clause map[string]string

	// GlobalsWritten maps a test function to package-level vars it assigns.
	GlobalsWritten map[string][]string
	// GlobalsRead maps a test function to package-level vars it reads.
	GlobalsRead map[string][]string

	// EnvReads is every literal key passed to os.Getenv / os.LookupEnv.
	EnvReads []string

	// EnvValues maps an env key to the string literals the code compares it
	// against (`os.Getenv("MODE") == "ci"` -> MODE: ["ci"]). The env probe
	// tries these values; without them it would have to guess, and guessing a
	// CI flag from a variable name is how tools report fantasies.
	EnvValues map[string][]string

	// FileCalls maps a test function to the os calls in it that leave state
	// outside the process ("os.WriteFile", "os.MkdirAll"). Static only: it says
	// THIS TEST TOUCHES DISK, never which path — the path is usually computed.
	FileCalls map[string][]string

	// HasTestMain reports a func TestMain(m *testing.M) in the package.
	HasTestMain bool
	// Subtests reports any t.Run usage in the package (report-facing aggregate).
	Subtests bool
	// TestSubtests reports t.Run usage per test function.
	//
	// Subtests are not a blocker: calling the parent test function is a faithful
	// way to reproduce its children, and the parent's verdict already folds
	// theirs in. The flag exists so a report can say "this witness runs a whole
	// subtest tree", not so the tool can refuse.
	TestSubtests map[string]bool
	// TestParallel reports t.Parallel() per test function.
	//
	// Used as a cheap gate: a package with no t.Parallel anywhere cannot have a
	// concurrency flake, so the parallel probe can answer "not this class"
	// without spending a single build.
	TestParallel map[string]bool

	// Files lists the parsed _test.go files, sorted.
	Files []string
}

// Clauses returns the distinct package clauses in the directory, sorted.
func (f PkgFacts) Clauses() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range f.Clause {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// SharedGlobals returns vars written by one test and read by another, which is
// the classic internal-pollution signature (39% of order-dependent failures in
// real-world scans). Sorted, deduped.
func (f PkgFacts) SharedGlobals() []string {
	read := map[string]bool{}
	for _, vars := range f.GlobalsRead {
		for _, v := range vars {
			read[v] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, vars := range f.GlobalsWritten {
		for _, v := range vars {
			if read[v] && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	sort.Strings(out)
	return out
}

// WriterOf returns the TEST functions that write the given package var, sorted.
//
// Helper functions are excluded on purpose: `reset()` in a fixture writes the
// same var, but only a test can be a polluter. Helpers still show up in
// GlobalsWritten for the evidence report.
func (f PkgFacts) WriterOf(name string) []string {
	var out []string
	for fn, vars := range f.GlobalsWritten {
		if _, isTest := f.Clause[fn]; !isTest {
			continue
		}
		for _, v := range vars {
			if v == name {
				out = append(out, fn)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
