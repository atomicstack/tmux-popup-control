package plugin

import "golang.org/x/sys/unix"

// publishClone atomically publishes a clone only when its destination is absent.
func publishClone(source, destination string) error {
	return unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
}
