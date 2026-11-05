package report_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/report"
)

func sample() domain.Capsule {
	return domain.Capsule{
		ID:   "fs_abc123456789",
		Kind: domain.KindOrder,
		Witness: domain.Witness{
			Kind:      domain.KindOrder,
			Victim:    domain.TestID{Package: "p", Name: "TestVictim"},
			Polluters: []domain.TestID{{Package: "p", Name: "TestPolluter"}},
			Runs:      5,
		},
		Observation: domain.Observation{Runs: 5, Failures: 5, Deterministic: true},
		Evidence: domain.Evidence{
			GlobalsWritten: map[string][]string{"TestPolluter": {"seeded"}},
			GlobalsRead:    map[string][]string{"TestVictim": {"seeded"}},
		},
		Repro:  domain.Repro{Command: "go test . -run '^TestFlakeWitness$'"},
		Status: domain.StatusConfirmed,
	}
}

func TestTextReportsTheEssentials(t *testing.T) {
	var b bytes.Buffer
	if err := report.Text(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"flake found", "TestVictim", "order", "TestPolluter", "5/5", "fs_abc123456789"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from output: %s", want, out)
		}
	}
}

// The single most valuable line: the writer/reader intersection. But it is never called a "root cause".
func TestTextShowsSharedGlobalAsObservation(t *testing.T) {
	var b bytes.Buffer
	if err := report.Text(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "TestPolluter writes, TestVictim reads: seeded") {
		t.Errorf("intersection line missing: %s", out)
	}
	if strings.Contains(strings.ToLower(out), "root cause") {
		t.Error("must not claim a root cause")
	}
}

func TestTextWithoutEvidence(t *testing.T) {
	c := sample()
	c.Evidence = domain.Evidence{}
	var b bytes.Buffer
	if err := report.Text(&b, c); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "evidence") {
		t.Error("the line must not print without evidence")
	}
}

// An empty result must be honest: what was tried, why nothing came of it.
func TestNoFlakeListsAttempts(t *testing.T) {
	var b bytes.Buffer
	err := report.NoFlake(&b, []report.AttemptInfo{
		{Probe: "repeat", Err: domain.ErrNotReproduced, NotReproduced: true},
		{Probe: "order", Err: errors.New("time ran out")},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "flake not found") {
		t.Errorf("title missing: %s", out)
	}
	if !strings.Contains(out, "repeat") || !strings.Contains(out, "order") {
		t.Errorf("tried probes must be listed: %s", out)
	}
	if !strings.Contains(out, "time ran out") {
		t.Errorf("the error reason must be shown: %s", out)
	}
}

func TestJSONIsMachineReadable(t *testing.T) {
	var b bytes.Buffer
	if err := report.JSON(&b, sample()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"schema"`) {
		t.Errorf("JSON fields missing: %s", b.String())
	}
}
