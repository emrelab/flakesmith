package probe_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/emrelab/go-flake-hunter/internal/probe"
)

// TestDefaultIsTheWholeProbeSet pins the single definition of the probe set:
// every implemented probe is registered, in cost order. A probe wired into the
// CLI but missing here is exactly the drift issue #14 described, so the order
// is asserted literally rather than as a set.
func TestDefaultIsTheWholeProbeSet(t *testing.T) {
	got := names(probe.Default(probe.Config{}))
	want := []string{"repeat", "order", "parallel", "env"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("default probe order = %v, want %v", got, want)
	}
}

// TestDefaultCrossIsLastAndOptIn asserts the two-binary engine keeps both
// properties it was designed with: it is the most expensive probe, so it runs
// last, and it costs nothing unless the caller passes a polluter package.
func TestDefaultCrossIsLastAndOptIn(t *testing.T) {
	if got := names(probe.Default(probe.Config{})); contains(got, "order-cross") {
		t.Fatalf("order-cross registered without a polluter dir: %v", got)
	}
	got := names(probe.Default(probe.Config{PolluterDir: "fixture/polluter"}))
	if len(got) == 0 || got[len(got)-1] != "order-cross" {
		t.Fatalf("order-cross must be last, got %v", got)
	}
}

// TestDefaultPassesTheConfigThrough asserts the other half of the single
// definition: Default owns the list, but the caller still owns the knobs. A
// transposed field here would silently hunt with the wrong -count or seeds and
// no order test would notice, so each value is read back off the probe itself.
func TestDefaultPassesTheConfigThrough(t *testing.T) {
	seeds := []int64{7, 11, 13}
	reg := probe.Default(probe.Config{Repeat: 3, Seeds: seeds})

	var repeat *probe.Repeat
	var order *probe.Order
	for _, p := range reg.Probes() {
		switch v := p.(type) {
		case probe.Repeat:
			repeat = &v
		case probe.Order:
			order = &v
		}
	}
	if repeat == nil || order == nil {
		t.Fatalf("Default dropped a probe: %v", names(reg))
	}
	if repeat.Count != 3 {
		t.Errorf("Repeat.Count = %d, want 3", repeat.Count)
	}
	if !slices.Equal(order.Seeds, seeds) {
		t.Errorf("Order.Seeds = %v, want %v", order.Seeds, seeds)
	}
}

func names(r *probe.Registry) []string {
	ps := r.Probes()
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name()
	}
	return out
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
