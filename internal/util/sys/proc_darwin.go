package sys

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"runtime"
)

// WaitUntilProcessFinished waits until the process exits or the context ends.
func WaitUntilProcessFinished(ctx context.Context, pid int) error {
	kq, err := unix.Kqueue()
	if err != nil {
		return err
	}
	defer unix.Close(kq)

	var (
		event = unix.Kevent_t{
			Ident:  uint64(pid),
			Filter: unix.EVFILT_PROC,
			Flags:  unix.EV_ADD,
			Fflags: unix.NOTE_EXIT,
		}
		events [32]unix.Kevent_t
	)

	_, err = unix.Kevent(kq, []unix.Kevent_t{event}, nil, nil)
	if errors.Is(err, unix.ESRCH) {
		return nil
	} else if err != nil {
		return err
	}

	for {
		n, errno := unix.Kevent(kq, nil, events[:], nil)
		switch errno {
		case nil:
			if n >= len(events) {
				return fmt.Errorf("kevent: returned more than %d events", n)
			} else if n > 0 {
				break
			}
			// No readiness event means the loop should continue waiting.
			fallthrough
		case unix.EINTR:
			runtime.Gosched()
			continue
		default:
			return errno
		}

		// Dispatch process exit readiness or context cancellation events.
		for _, e := range events[:n] {
			if e.Ident == uint64(pid) && (e.Fflags&unix.NOTE_EXIT) != 0 {
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

// SetProcessName sets the visible process name when the platform supports it.
func SetProcessName(_ string) error {
	return errors.New("SetProcessName is not supported")
}
