package appglobals

import "testing"

func TestPolluter(t *testing.T) { Pollute() }
func TestVictim(t *testing.T) {
	if state != 0 {
		t.Fatal("shared app state polluted")
	}
}
