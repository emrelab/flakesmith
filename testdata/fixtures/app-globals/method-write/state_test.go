package appglobals

import "testing"

func TestPolluter(t *testing.T) { state.Pollute() }
func TestVictim(t *testing.T) {
	if state.Value() != 0 {
		t.Fatal("shared app state polluted")
	}
}
