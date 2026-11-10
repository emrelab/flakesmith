package substitutionleak

import "testing"

func TestDefaultSubstitution(t *testing.T) {
	if got := normalize("tea & coffee"); got != "tea and coffee" {
		t.Fatalf("unexpected substitution: %s", got)
	}
}

func TestCustomSubstitution(t *testing.T) {
	replacement = "plus"
	if got := normalize("tea & coffee"); got != "tea plus coffee" {
		t.Fatalf("unexpected custom substitution: %s", got)
	}
}
