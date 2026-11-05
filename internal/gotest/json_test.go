package gotest

import (
	"testing"

	"github.com/emrelab/flakesmith/internal/domain"
)

// A sample trimmed from a real `go test -json` stream: one test passes, one
// fails, with subtest and package-level events in between.
const streamOnePassOneFail = `{"Time":"2026-11-02T01:00:00Z","Action":"start","Package":"fixture/x"}
{"Action":"run","Package":"fixture/x","Test":"TestPass"}
{"Action":"output","Package":"fixture/x","Test":"TestPass","Output":"=== RUN   TestPass\n"}
{"Action":"pass","Package":"fixture/x","Test":"TestPass","Elapsed":0.01}
{"Action":"run","Package":"fixture/x","Test":"TestFail"}
{"Action":"output","Package":"fixture/x","Test":"TestFail","Output":"    x_test.go:9: blew up\n"}
{"Action":"fail","Package":"fixture/x","Test":"TestFail","Elapsed":0.02}
{"Action":"fail","Package":"fixture/x","Elapsed":0.04}
`

func TestParseEventsBasic(t *testing.T) {
	spec := domain.RunSpec{Package: "./x", Count: 1}
	res, err := parseEvents([]byte(streamOnePassOneFail), spec, 1, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed("TestPass") {
		t.Error("TestPass should have passed")
	}
	if !res.Failed("TestFail") {
		t.Error("TestFail should have failed")
	}
	if got, want := len(res.Order), 2; got != want {
		t.Errorf("Order = %v, want %d elements", res.Order, want)
	}
	if res.Order[0] != "TestPass" || res.Order[1] != "TestFail" {
		t.Errorf("wrong Order sequence: %v", res.Order)
	}
	if res.Outcomes["TestFail"].Output == "" {
		t.Error("failing test output must be recorded")
	}
}

// -count=2: the same test passes first, then fails. The WORST status must win,
// or the repeat probe misses the flake.
const streamPassThenFail = `{"Action":"run","Test":"TestAccum"}
{"Action":"pass","Test":"TestAccum","Elapsed":0.01}
{"Action":"run","Test":"TestAccum"}
{"Action":"fail","Test":"TestAccum","Elapsed":0.01}
`

func TestParseEventsWorstStatusWins(t *testing.T) {
	res, err := parseEvents([]byte(streamPassThenFail), domain.RunSpec{Count: 2}, 1, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed("TestAccum") {
		t.Fatalf("a test failing in round 2 must be fail, got: %v", res.Outcomes["TestAccum"].Status)
	}
	if len(res.Order) != 1 {
		t.Errorf("the same test must not be added to Order twice: %v", res.Order)
	}
}

// Subtests must stay out of Order: parallel tests scramble the sequence.
const streamWithSubtests = `{"Action":"run","Test":"TestParent"}
{"Action":"run","Test":"TestParent/alt1"}
{"Action":"pass","Test":"TestParent/alt1"}
{"Action":"pass","Test":"TestParent"}
`

func TestParseEventsSkipsSubtestsInOrder(t *testing.T) {
	res, err := parseEvents([]byte(streamWithSubtests), domain.RunSpec{Count: 1}, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Order) != 1 || res.Order[0] != "TestParent" {
		t.Errorf("Order must contain only top-level tests: %v", res.Order)
	}
	if !res.Passed("TestParent/alt1") {
		t.Error("subtest result must be recorded (just not in Order)")
	}
}

func TestParseEventsSkipsNoise(t *testing.T) {
	// `go test` may print plain text before the JSON; a corrupt line must not crash the stream.
	in := "warning: something\n{broken json\n" + streamOnePassOneFail
	res, err := parseEvents([]byte(in), domain.RunSpec{Count: 1}, 1, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outcomes) != 2 {
		t.Errorf("corrupt lines must be skipped, found %d results", len(res.Outcomes))
	}
}
