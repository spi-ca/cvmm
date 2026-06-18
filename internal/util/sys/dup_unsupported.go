//go:build !unix || windows
// +build !unix windows

package sys

// ReplaceFD atomically replaces one file descriptor with another when the platform supports it.
func ReplaceFD(oldfd int, newfd int) (err error) {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}
