package gotest

import "testing"

func TestNormalizeCollapsesNoise(t *testing.T) {
	a := "2026/11/02 01:04:33 startup failed: /tmp/abc123/x.db (0xc000123456) 12.5ms 127.0.0.1:54321"
	b := "2026/11/02 01:09:59 startup failed: /tmp/zzz999/x.db (0xc000987654) 8ms 127.0.0.1:61001"

	na, nb := Normalize(a), Normalize(b)
	if na != nb {
		t.Fatalf("same error normalized differently:\n a=%q\n b=%q", na, nb)
	}
}

func TestNormalizeKeepsMeaning(t *testing.T) {
	in := "shared state polluted: seeded=42"
	if got := Normalize(in); got != in {
		t.Errorf("meaningful text must not be changed, got: %q", got)
	}
}

func TestNormalizeEmpty(t *testing.T) {
	if Normalize("") != "" {
		t.Error("empty input must return empty")
	}
}
