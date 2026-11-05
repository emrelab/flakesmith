//go:build windows

package policy

import "os"

// Windows has no flock in the stdlib; the in-process mutex still serializes
// this binary, which is the common case (one hunt at a time).
func flockExclusive(f *os.File) error { return nil }

func funlock(f *os.File) error { return nil }
