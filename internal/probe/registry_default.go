package probe

// Config carries the per-run knobs the CLI exposes as flags. The probe list
// itself is NOT configurable: it lives only in Default, so adding a probe is a
// one-place edit and the two halves of the product cannot drift apart.
type Config struct {
	// Repeat is Repeat.Count; 0 -> DefaultRepeatCount.
	Repeat int
	// Seeds is Order.Seeds; nil -> DefaultSeeds.
	Seeds []int64
	// PolluterDir is the opt-in two-binary engine's foreign package. Empty
	// means that probe is not registered at all.
	PolluterDir string
}

// Default returns the registry with every probe implemented so far, in cost
// order. This is the one definition of the probe set: the order of this slice
// IS the cost ordering the product depends on, and adding a probe requires
// touching this function only — never the CLI.
func Default(c Config) *Registry {
	probes := []Probe{
		Repeat{Count: c.Repeat}, // 1 run: cheapest
		Order{Seeds: c.Seeds},   // tens of runs: order dependence
		Parallel{},
		Env{},
	}
	if c.PolluterDir != "" {
		// The two-binary engine (#6) is last and opt-in: it is the most
		// expensive probe and its search space is a whole foreign package.
		probes = append(probes, OrderCross{PolluterDir: c.PolluterDir})
	}
	return NewRegistry(probes...)
}
