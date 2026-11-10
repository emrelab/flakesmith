package appglobals

import "testing"

func TestPolluter(t *testing.T) { state = 1 }
func TestVictim(t *testing.T) {
	if Value() != 0 {
		t.Fatal("shared app state polluted")
	}
}
