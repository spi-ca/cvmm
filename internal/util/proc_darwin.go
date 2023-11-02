package util

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"runtime"
)

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
			// if n <=0
			fallthrough
		case unix.EINTR:
			runtime.Gosched()
			continue
		default:
			return errno
		}

		// Process events
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

func SetProcessName(_ string) error {
	return errors.New("SetProcessName is not supported")
}
