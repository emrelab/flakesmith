package facts

import "sort"

// Pos is where a test function is declared.
type Pos struct {
	// File is the absolute path of the declaring file.
	File string
	// Line is the 1-based line of the func keyword.
	Line int
}

// PkgFacts is the static knowledge sheet for one test package directory.
type PkgFacts struct {
	// ExternalDependencies are package-wide advisory call sites, not proven causes.
	ExternalDependencies []string

	// Dir is the package directory as given to Scan.
	Dir string
	// ImportPath is resolved from go.mod-relative position when available.
	ImportPath string

	// Clause maps a test function name to its file's package clause
	// ("flakedemo" or "flakedemo_test").
	//
	// The two clauses are ONE test binary (cmd/go: the `_test`-suffixed
	// package is compiled separately and LINKED INTO the main test binary), so
	// in-memory state does cross between them. What a clause boundary does
	// block is the witness form: a generated file lives in one clause and
	// cannot call a test function of the other, so a witness spanning them must
	// be expressed as a single `-run` invocation instead of a t.Run chain.
	Clause map[string]string

	// TestPos is the declaration site of every test function the scan saw.
	// SARIF annotations need file+line; the AST scan already has it, so it is
	// free. Missing entry = the scan did not see that test (excluded by a
	// build tag, or not in this directory).
	TestPos map[string]Pos

	// GlobalsWritten maps a test function to package-level vars it assigns.
	GlobalsWritten map[string][]string
	// GlobalsRead maps a test function to package-level vars it reads.
	GlobalsRead map[string][]string

	// EnvReads is every literal key passed to os.Getenv / os.LookupEnv in the
	// package's _test.go files.
	EnvReads []string

	// AppEnvReads is every literal key passed to os.Getenv / os.LookupEnv in
	// the package's NON-test .go files (methods included). A test can flip on
	// an env value it never reads itself: the application code reads it and
	// the test exercises that path. Probe candidate order is test-local keys
	// first, these second — app code is noisier and candidate order is
	// budget order. Comparison literals land in EnvValues like test keys.
	AppEnvReads []string

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
	// TestSynctest reports testing/synctest usage per test function: a call to
	// synctest.Test (Go 1.26) / synctest.Run / synctest.Omega / synctest.Wait /
	// synctest.Go anywhere in its body (bubbles opened by a called helper are
	// invisible — static scope, same as FileCalls).
	//
	// The point is naming, not fixing: a bubble provides a virtual clock,
	// but it does not isolate external I/O or make arbitrary code deterministic. go-flake-hunter never synthesizes bubbles around
	// someone else's test: that would fabricate determinism.
	TestSynctest map[string]bool

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
