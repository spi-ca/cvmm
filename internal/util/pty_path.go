package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

var consolePTYPathPattern = regexp.MustCompile(`^/dev/pts/[0-9]+$`)

// ValidateConsolePTYPath rejects non-canonical or non-PTY console device paths.
func ValidateConsolePTYPath(ptyPath string) error {
	info, err := statConsolePTYPath(ptyPath)
	if err != nil {
		return err
	}
	return validateConsolePTYPathInfo(ptyPath, info, 0, false)
}

// ValidateDirectConsolePTYPath applies additional ownership checks for console-file direct PTY attaches.
func ValidateDirectConsolePTYPath(ptyPath string) error {
	info, err := statConsolePTYPath(ptyPath)
	if err != nil {
		return err
	}
	euid := os.Geteuid()
	return validateConsolePTYPathInfo(ptyPath, info, euid, euid != 0)
}

func statConsolePTYPath(ptyPath string) (os.FileInfo, error) {
	if len(ptyPath) == 0 {
		return nil, fmt.Errorf("invalid console PTY path %q", ptyPath)
	}
	if cleaned := filepath.Clean(ptyPath); cleaned != ptyPath {
		return nil, fmt.Errorf("invalid console PTY path %q", ptyPath)
	}
	if !consolePTYPathPattern.MatchString(ptyPath) {
		return nil, fmt.Errorf("invalid console PTY path %q", ptyPath)
	}

	info, err := os.Stat(ptyPath)
	if err != nil {
		return nil, fmt.Errorf("invalid console PTY path %q: %w", ptyPath, err)
	}
	return info, nil
}

func validateConsolePTYPathInfo(ptyPath string, info os.FileInfo, currentEUID int, requireCurrentOwner bool) error {
	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("invalid console PTY path %q: not a character device", ptyPath)
	}
	if !requireCurrentOwner {
		return nil
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return fmt.Errorf("invalid console PTY path %q: missing ownership metadata", ptyPath)
	}
	if int(stat.Uid) != currentEUID {
		return fmt.Errorf("invalid console PTY path %q: direct attach requires PTY owner euid %d", ptyPath, currentEUID)
	}
	return nil
}
