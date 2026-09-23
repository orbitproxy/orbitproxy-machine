//go:build unix

package runlock

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockExclusive(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
