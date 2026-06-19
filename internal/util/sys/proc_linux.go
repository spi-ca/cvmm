package sys

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const waitUntilProcessFinishedPollInterval = 100 * time.Millisecond

// WaitUntilProcessFinished waits until the process exits or the context ends.
func WaitUntilProcessFinished(ctx context.Context, pid int) error {
	pidfd, err := unix.PidfdOpen(pid, 0)
	switch {
	case err == nil:
		defer unix.Close(pidfd)
		return waitUntilProcessFinishedWithPidfd(ctx, pidfd)
	case errors.Is(err, os.ErrNotExist), errors.Is(err, unix.ESRCH):
		return nil
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EINVAL), errors.Is(err, unix.EPERM):
		return waitUntilProcessFinishedWithProc(ctx, pid)
	case err != nil:
		return err
	}

	return waitUntilProcessFinishedWithProc(ctx, pid)
}

func waitUntilProcessFinishedWithPidfd(ctx context.Context, pidfd int) error {
	epollfd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return err
	}
	defer unix.Close(epollfd)

	event := unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLERR | unix.EPOLLHUP, Fd: int32(pidfd)}
	if err := unix.EpollCtl(epollfd, unix.EPOLL_CTL_ADD, pidfd, &event); err != nil {
		return err
	}

	events := make([]unix.EpollEvent, 1)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		n, err := unix.EpollWait(epollfd, events, waitUntilProcessFinishedTimeoutMillis(ctx))
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		if n > 0 {
			return nil
		}
	}
}

func waitUntilProcessFinishedWithProc(ctx context.Context, pid int) error {
	_, expectedStartTime, err := readProcessStateAndStartTime(pid)
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, unix.ESRCH):
		return nil
	case err != nil:
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		state, startTime, err := readProcessStateAndStartTime(pid)
		switch {
		case errors.Is(err, os.ErrNotExist), errors.Is(err, unix.ESRCH):
			return nil
		case err != nil:
			return err
		case startTime != expectedStartTime:
			return nil
		case state == 'Z':
			return nil
		}

		if err := waitUntilProcessFinishedSleep(ctx); err != nil {
			return err
		}
	}
}

func waitUntilProcessFinishedTimeoutMillis(ctx context.Context) int {
	timeoutMillis := int(waitUntilProcessFinishedPollInterval / time.Millisecond)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 1
		}
		if remaining < waitUntilProcessFinishedPollInterval {
			timeoutMillis = int((remaining + time.Millisecond - 1) / time.Millisecond)
			if timeoutMillis <= 0 {
				timeoutMillis = 1
			}
		}
	}
	return timeoutMillis
}

func waitUntilProcessFinishedSleep(ctx context.Context) error {
	timer := time.NewTimer(time.Duration(waitUntilProcessFinishedTimeoutMillis(ctx)) * time.Millisecond)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

const linuxProcessNameLimit = 15

func readProcessName(pid int) (string, bool, error) {
	buf, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(buf)), true, nil
}

func readProcessCmdline(pid int) ([]string, bool, error) {
	buf, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, false, err
	}
	if len(buf) == 0 {
		return []string{}, true, nil
	}

	parts := strings.Split(string(buf), "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts, true, nil
}

func readProcessStateAndStartTime(pid int) (byte, uint64, error) {
	buf, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, err
	}

	text := strings.TrimSpace(string(buf))
	end := strings.LastIndex(text, ")")
	if end < 0 || end+2 > len(text) {
		return 0, 0, fmt.Errorf("unexpected /proc/%d/stat format", pid)
	}

	fields := strings.Fields(text[end+2:])
	if len(fields) <= 19 {
		return 0, 0, fmt.Errorf("unexpected /proc/%d/stat field count", pid)
	}

	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("unexpected /proc/%d/stat starttime: %w", pid, err)
	}
	if len(fields[0]) == 0 {
		return 0, 0, fmt.Errorf("unexpected /proc/%d/stat state", pid)
	}

	return fields[0][0], startTime, nil
}

func normalizeProcessName(name string) string {
	if len(name) <= linuxProcessNameLimit {
		return name
	}
	return name[:linuxProcessNameLimit]
}

// SetProcessName sets the visible process name when the platform supports it.
func SetProcessName(name string) error {
	strptr, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}

	ptr := uintptr(unsafe.Pointer(strptr))
	return unix.Prctl(unix.PR_SET_NAME, ptr, 0, 0, 0)
}
