package probe

import "errors"

// errorsIs is a thin alias so probe.go reads cleanly and stays stdlib-only.
func errorsIs(err, target error) bool { return errors.Is(err, target) }
