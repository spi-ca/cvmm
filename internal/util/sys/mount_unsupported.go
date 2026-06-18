//go:build !linux
// +build !linux

package sys

import (
	"fmt"
	"runtime"
)

// Sandbox applies the platform sandbox mount setup for the current process.
func Sandbox(_ string) error {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}

// Mount creates the destination directory and mounts the requested source there.
func Mount(_ string, _ string, _ string, _ string) (err error) {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}

// Umount unmounts the requested destination path.
func Umount(_ string) error {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}

// RecursiveUmount reports unsupported recursive unmount behavior on this platform.
func RecursiveUmount(_ string) error {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}
