//go:build !darwin && !linux

package plugin

import "errors"

func publishClone(source, destination string) error {
	return errors.New("safe plugin publication is unsupported on this platform")
}
