//go:build !windows

package policy

import (
	"os"
	"syscall"
)

// flockExclusive takes an exclusive advisory lock, blocking until it is free.
// Advisory locks are enough here: every participant is a flakesmith process.
func flockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func funlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
