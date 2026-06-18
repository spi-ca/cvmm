package util

import (
	"os/exec"
	"path/filepath"
)

// LookupBinary resolves a binary name or path to an absolute executable path.
func LookupBinary(givenPath string) string {
	if len(givenPath) < 1 {
		return ""
	}

	if foundPath, err := exec.LookPath(givenPath); err != nil {
		ErrLog.Printf("binary(%s) not found", givenPath)
		return ""
	} else {
		absPath, _ := filepath.Abs(foundPath)
		return absPath
	}
}
