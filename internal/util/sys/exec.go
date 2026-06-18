package sys

import (
	"fmt"
	"os"
)

var (
	selfExecutablePath string
)

// init prepares package-level defaults before the package is used.
func init() {
	if exePath, err := os.Executable(); err != nil {
		panic(fmt.Errorf("failed to get self-path: %w", err))
	} else {
		selfExecutablePath = exePath
	}
}

// Executable returns the current executable path captured during package initialization.
func Executable() string {
	return selfExecutablePath
}
