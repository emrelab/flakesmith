package probe

// Default returns the registry with the probes implemented so far, in cost
// order. Later tasks append here (order: T8, parallel: T11, env: T11); the
// order of this slice IS the cost ordering the product depends on.
func Default() *Registry {
	return NewRegistry(
		Repeat{}, // 1 run: cheapest
		Order{},  // tens of runs: order dependence
	)
}
