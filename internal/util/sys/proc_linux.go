package sys

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func WaitUntilProcessFinished(ctx context.Context, pid int) error {
	fd, err := unix.PidfdOpen(pid, unix.PIDFD_NONBLOCK)
	if errors.Is(err, unix.ESRCH) {
		return nil
	} else if err != nil {
		return err
	}
	defer unix.Close(fd)

	epfd, e := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if e != nil {
		return fmt.Errorf("epoll_create1: %w", e)
	}
	defer unix.Close(epfd)

	var (
		event = unix.EpollEvent{
			Events: unix.EPOLLIN | unix.EPOLLET,
			Fd:     int32(fd),
		}
		events [32]unix.EpollEvent
	)

	if e = unix.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); e != nil {
		return fmt.Errorf("epoll_ctl: %w", e)
	}

	msec := -1

	// Loop forever
	for {
		// Poll the file descriptor
		n, errno := unix.EpollWait(epfd, events[:], msec)
		switch errno {
		case nil:
			if n >= len(events) {
				return fmt.Errorf("epoll_wait: returned more than %d events", n)
			} else if n > 0 {
				msec = 0
				break
			}
			// if n <=0
			fallthrough
		case unix.EINTR:
			runtime.Gosched()
			msec = -1
			continue
		default:
			return errno
		}

		// Process events
		for _, e := range events[:n] {
			if e.Fd == int32(fd) && (e.Events&unix.EPOLLIN) != 0 {
				return nil
			}
		}

		// Check for done signal
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func SetProcessName(name string) error {
	//bytes := append([]byte(name), 0)
	//ptr := unsafe.Pointer(&bytes[0])

	strptr, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}

	ptr := uintptr(unsafe.Pointer(strptr))
	return unix.Prctl(unix.PR_SET_NAME, ptr, 0, 0, 0)
}
