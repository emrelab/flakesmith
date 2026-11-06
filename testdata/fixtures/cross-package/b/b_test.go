package b

import (
	"os"
	"strings"
	"testing"
)

// TestVictim passes on its own and fails only right after package a ran:
// it CONSUMES the pollution (resets the file to "clean"), which keeps the
// witness re-flakable per round instead of one-shot, and makes the alone-run
// pass from a fresh checkout.
func TestVictim(t *testing.T) {
	const state = "../a/pollution.txt"
	data, err := os.ReadFile(state)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == "dirty" {
		_ = os.WriteFile(state, []byte("clean"), 0o644)
		t.Fatal("victim ran after the polluter: shared state was dirty")
	}
	if err := os.WriteFile(state, []byte("clean"), 0o644); err != nil {
		t.Fatal(err)
	}
}
