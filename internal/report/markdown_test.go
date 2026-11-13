package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/domain"
	"github.com/emrelab/go-flake-hunter/internal/report"
)

func capsule() domain.Capsule {
	return domain.Capsule{
		Schema: domain.SchemaVersion,
		ID:     "gfh_abc123456789",
		Kind:   domain.KindOrder,
		Target: domain.Target{Package: "example.com/svc", RelDir: ".", Victim: "TestCheckout"},
		Witness: domain.Witness{
			Kind:      domain.KindOrder,
			Victim:    domain.TestID{Package: "example.com/svc", Name: "TestCheckout"},
			Polluters: []domain.TestID{{Name: "TestSeedDB"}},
			Flags:     domain.Flags{Parallel: 1},
		},
		Observation: domain.Observation{Runs: 3, Failures: 3, Deterministic: true},
		Evidence: domain.Evidence{
			GlobalsWritten: map[string][]string{"TestSeedDB": {"seeded"}},
			GlobalsRead:    map[string][]string{"TestCheckout": {"seeded"}},
		},
		Repro:  domain.Repro{Command: "go test . -run '^TestFlakeWitness$' -count=1 -parallel=1"},
		Status: domain.StatusConfirmed,
	}
}

// Markdown must be a table: the one format read in PR comments and CI summaries.
func TestMarkdownIsATable(t *testing.T) {
	var b bytes.Buffer
	if err := report.Markdown(&b, capsule()); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	for _, want := range []string{
		"### flake: TestCheckout",
		"| field | value |",
		"|---|---|",
		"| class | order |",
		"| polluter | TestSeedDB |",
		"| evidence | TestSeedDB writes, TestCheckout reads: seeded |",
		"```sh",
		"gfh_abc123456789",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("%q missing from markdown:\n%s", want, s)
		}
	}
	// Table rows must be balanced: a missing pipe breaks the table on GitHub.
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "|") && strings.Count(line, "|") != 3 {
			t.Errorf("broken table row: %q", line)
		}
	}
}

// If the evidence text contains a pipe it must be escaped, or the row splits.
func TestMarkdownEscapesPipe(t *testing.T) {
	c := capsule()
	c.Evidence.GlobalsWritten = map[string][]string{"TestSeedDB": {"a|b"}}
	c.Evidence.GlobalsRead = map[string][]string{"TestCheckout": {"a|b"}}
	var b bytes.Buffer
	if err := report.Markdown(&b, c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `a\|b`) {
		t.Errorf("pipe not escaped:\n%s", b.String())
	}
}

// A partial capsule must show note and status: the answer to "why no result".
func TestMarkdownShowsPartialNote(t *testing.T) {
	c := capsule()
	c.Status = domain.StatusPartial
	c.Note = "budget exceeded: 1/1 run"
	c.Witness = domain.Witness{}
	var b bytes.Buffer
	if err := report.Markdown(&b, c); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if !strings.Contains(s, "| status | partial |") || !strings.Contains(s, "budget exceeded") {
		t.Errorf("partial info missing:\n%s", s)
	}
	if !strings.Contains(s, "(partial)") {
		t.Errorf("status missing from title:\n%s", s)
	}
}
