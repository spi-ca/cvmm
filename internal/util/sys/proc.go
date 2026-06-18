package sys

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// AcquirePidFile writes a pid file when no active process already owns it and returns a cleanup function.
func AcquirePidFile(filename string, pid int) (func(), error) {
	if IsPidFileActive(filename) {
		return nil, fmt.Errorf("already running(%s)", filename)
	}

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		// file exists
		if IsPidFileActive(filename) {
			return nil, fmt.Errorf("already running(%s)", filename)
		}
	}

	dirpath := filepath.Dir(filename)
	err := os.MkdirAll(dirpath, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to make dir of pidfile(%s): %w", dirpath, err)
	}

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create pid file(%s): %w", filename, err)
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
		_ = f.Close()
		_ = os.Remove(filename)
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
	f, err := os.OpenFile(filename, os.O_RDONLY, 0o644)
	if err != nil {
		return -1, fmt.Errorf("failed to crated pidfile(%s): %w", filename, err)
	}

	defer f.Close()

	reader := bufio.NewReader(f)

	line, isPrefix, err := reader.ReadLine()
	if err != nil {
		return -1, fmt.Errorf("failed to read pidfile(%s): %w", filename, err)
	} else if isPrefix {
		return -1, fmt.Errorf("first line is too long, pidfile(%s)", filename)
	}

	return strconv.Atoi(string(line))
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
