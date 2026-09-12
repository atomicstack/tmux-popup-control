package plugin

import "golang.org/x/sys/unix"

// publishClone atomically publishes a clone only when its destination is absent.
func publishClone(source, destination string) error {
	return unix.RenamexNp(source, destination, unix.RENAME_EXCL)
}
