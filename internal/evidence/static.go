// Package evidence turns static facts into the observation list a capsule
// carries.
//
// Evidence is an observation list, not a "because" claim: "this test writes that
// variable, that test reads it", "this test touches disk". We do not assert a root
// cause; we say just enough for a human to recognise the bug in a report.
// Contract: docs/ARCHITECTURE.md §8.
package evidence

import (
	"sort"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/facts"
)

// Build assembles the evidence for a witness.
//
// Attribution rule: only the POLLUTERS' effects are recorded. A victim that
// writes a global or touches a file is not evidence of pollution — it is the
// victim minding its own business. Including it would make every capsule look
// like it found the same cause.
func Build(w *domain.Witness, f facts.PkgFacts) domain.Evidence {
	ev := domain.Evidence{EnvDiff: w.EnvDiff, ExternalDependencies: f.ExternalDependencies}
	if len(w.Polluters) == 0 {
		return ev
	}

	written := map[string][]string{}
	read := map[string][]string{}
	for _, p := range w.Polluters {
		if v := f.GlobalsWritten[p.Name]; len(v) > 0 {
			written[p.Name] = v
		}
		if v := f.FileCalls[p.Name]; len(v) > 0 {
			ev.FilesTouched = append(ev.FilesTouched, p.Name+" → "+join(v))
		}
	}
	for _, name := range victimAndPolluters(w) {
		if v := f.GlobalsRead[name]; len(v) > 0 {
			read[name] = v
		}
	}
	if len(written) > 0 {
		ev.GlobalsWritten = written
	}
	if len(read) > 0 {
		ev.GlobalsRead = read
	}
	sort.Strings(ev.FilesTouched)
	return ev
}

// Shared is the intersection the report shows first: a variable one test writes
// and another reads. It is the single most useful line in a capsule, which is
// why it is computed here rather than left to each renderer.
type Shared struct {
	Var    string
	Writer string
	Reader string
}

// SharedGlobals returns writer→reader pairs for package vars touched by
// different tests, sorted for stable output.
//
// The writer pick is deterministic on purpose. Iterating the map and taking the
// first writer made the same package report TestPolluter on one run and reset
// (the helper TestPolluter calls) on the next — a report that changes without a
// code change, and a test suite that fails only sometimes. Real tests win over
// helpers; ties break alphabetically.
func SharedGlobals(f facts.PkgFacts) []Shared {
	writer := map[string]string{}
	for test, vars := range f.GlobalsWritten {
		for _, v := range vars {
			if cur, ok := writer[v]; !ok || preferWriter(test, cur) {
				writer[v] = test
			}
		}
	}
	var out []Shared
	for test, vars := range f.GlobalsRead {
		for _, v := range vars {
			if w, ok := writer[v]; ok && w != test {
				out = append(out, Shared{Var: v, Writer: w, Reader: test})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Writer != out[j].Writer {
			return out[i].Writer < out[j].Writer
		}
		return out[i].Var < out[j].Var
	})
	return out
}

// preferWriter reports whether candidate should represent the variable instead
// of current. A Test function beats a helper it calls; otherwise the lexical
// order decides, so the answer never depends on map iteration.
func preferWriter(candidate, current string) bool {
	cTest := strings.HasPrefix(candidate, "Test")
	curTest := strings.HasPrefix(current, "Test")
	if cTest != curTest {
		return cTest
	}
	return candidate < current
}

func victimAndPolluters(w *domain.Witness) []string {
	out := []string{w.Victim.Name}
	for _, p := range w.Polluters {
		out = append(out, p.Name)
	}
	return out
}

func join(v []string) string {
	s := ""
	for i, x := range v {
		if i > 0 {
			s += ", "
		}
		s += x
	}
	return s
}
