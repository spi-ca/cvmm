package sys

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

type ProcessIdentityStatus int

type ProcessIdentityExpectation struct {
	Name               string
	ExecutableBasename string
	CommandArgs        []string
}

const (
	ProcessIdentityInactive ProcessIdentityStatus = iota
	ProcessIdentityMatch
	ProcessIdentityMismatch
	ProcessIdentityUnknown
)

// AcquirePidFile writes a pid file when no active process already owns it and returns a cleanup function.
func AcquirePidFile(filename string, pid int) (func(), error) {
	return acquirePidFile(filename, pid, false)
}

// AcquirePidFileReplacing writes a pid file after the caller has already verified that any existing owner is stale or unrelated.
func AcquirePidFileReplacing(filename string, pid int) (func(), error) {
	return acquirePidFile(filename, pid, true)
}

func acquirePidFile(filename string, pid int, replaceActive bool) (func(), error) {
	if !replaceActive && IsPidFileActive(filename) {
		return nil, fmt.Errorf("already running(%s)", filename)
	}

	if !replaceActive {
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			// file exists
			if IsPidFileActive(filename) {
				return nil, fmt.Errorf("already running(%s)", filename)
			}
		}
	}

	dirpath := filepath.Dir(filename)
	err := os.MkdirAll(dirpath, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to make dir of pidfile(%s): %w", dirpath, err)
	}

	fd, err := unix.Open(filename, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to create pid file(%s): %w", filename, err)
	}
	f := os.NewFile(uintptr(fd), filename)
	if f == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed to create pid file handle(%s)", filename)
	}
	pidFileInfo, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to stat pidfile(%s): %w", filename, err)
	}
	if !pidFileInfo.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("pidfile(%s) is not a regular file", filename)
	}

	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to lock pidfile(%s): %w", filename, err)
	}

	_, err = f.Seek(0, io.SeekCurrent)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to seek pidfile(%s): %w", filename, err)
	}

	err = f.Truncate(0)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to truncate pidfile(%s): %w", filename, err)
	}

	_, err = f.WriteString(fmt.Sprintf("%d\n", pid))
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to write pidfile(%s): %w", filename, err)
	}

	return func() {
		if currentInfo, statErr := os.Lstat(filename); statErr == nil && currentInfo.Mode().IsRegular() && os.SameFile(currentInfo, pidFileInfo) {
			_ = os.Remove(filename)
		}
		_ = f.Close()
	}, nil
}

// IsPidFileActive reports whether a pid file points to a currently running process.
func IsPidFileActive(filename string) bool {
	if oldpid, _ := ReadPidFile(filename); oldpid > 0 {
		return IsPidActive(oldpid)
	}
	return false
}

// ReadPidFile parses a process id from a pid file.
func ReadPidFile(filename string) (int, error) {
	pid, _, err := ReadPidFileInfo(filename)
	return pid, err
}

// ReadPidFileInfo parses a process id from a pid file and returns the opened file identity.
func ReadPidFileInfo(filename string) (int, os.FileInfo, error) {
	fd, err := unix.Open(filename, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, nil, fmt.Errorf("failed to open pidfile(%s): %w", filename, err)
	}
	f := os.NewFile(uintptr(fd), filename)
	if f == nil {
		_ = unix.Close(fd)
		return -1, nil, fmt.Errorf("failed to create pid file handle(%s)", filename)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return -1, nil, fmt.Errorf("failed to stat pidfile(%s): %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		return -1, nil, fmt.Errorf("pidfile(%s) is not a regular file", filename)
	}

	reader := bufio.NewReader(f)

	line, isPrefix, err := reader.ReadLine()
	if err != nil {
		return -1, nil, fmt.Errorf("failed to read pidfile(%s): %w", filename, err)
	} else if isPrefix {
		return -1, nil, fmt.Errorf("first line is too long, pidfile(%s)", filename)
	}

	pid, err := strconv.Atoi(string(line))
	return pid, info, err
}

// IsPidActive reports whether a process id can be signaled on the current host.
func IsPidActive(pid int) bool {
	if pid <= 0 {
		return false
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return p.Signal(os.Signal(syscall.Signal(0))) == nil
}

// ProcessIdentity reports whether pid is still running and whether its visible process name matches the expected one.
func ProcessIdentity(pid int, expectedName string) (ProcessIdentityStatus, error) {
	return ProcessIdentityWithExpectation(pid, ProcessIdentityExpectation{Name: expectedName})
}

// ProcessIdentityWithExpectation reports whether pid is running and matches the expected process name and command line details.
func ProcessIdentityWithExpectation(pid int, expected ProcessIdentityExpectation) (ProcessIdentityStatus, error) {
	if !IsPidActive(pid) {
		return ProcessIdentityInactive, nil
	}

	actualName, known, err := readProcessName(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			return ProcessIdentityInactive, nil
		}
		return ProcessIdentityUnknown, err
	}
	if !known {
		return ProcessIdentityUnknown, nil
	}
	if len(expected.Name) > 0 && actualName != normalizeProcessName(expected.Name) {
		return ProcessIdentityMismatch, nil
	}

	if len(expected.ExecutableBasename) == 0 && len(expected.CommandArgs) == 0 {
		return ProcessIdentityMatch, nil
	}

	cmdline, known, err := readProcessCmdline(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			return ProcessIdentityInactive, nil
		}
		return ProcessIdentityUnknown, err
	}
	if !known {
		return ProcessIdentityUnknown, nil
	}
	if len(expected.ExecutableBasename) > 0 {
		if len(cmdline) == 0 || filepath.Base(cmdline[0]) != expected.ExecutableBasename {
			return ProcessIdentityMismatch, nil
		}
	}
	if len(expected.CommandArgs) > 0 {
		if len(cmdline) == 0 || !containsOrderedArgs(cmdline[1:], expected.CommandArgs) {
			return ProcessIdentityMismatch, nil
		}
	}

	return ProcessIdentityMatch, nil
}

func containsOrderedArgs(haystack, needle []string) bool {
	if len(needle) == 0 {
		return true
	}

	needleIdx := 0
	for _, candidate := range haystack {
		if candidate != needle[needleIdx] {
			continue
		}
		needleIdx++
		if needleIdx == len(needle) {
			return true
		}
	}
	return false
}
