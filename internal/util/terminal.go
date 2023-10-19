package util

import (
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"os"
	"os/signal"
)

func PrepareTerminal(outer, inner int) func() {
	_, _ = unix.Write(inner, []byte{'\n'})

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(outer)
	if err != nil {
		ErrLog.Printf("failed to initializing pty: %s", err)
		return func() {}
	}

	ch := make(chan os.Signal, 1)
	ch <- unix.SIGWINCH // Initial resize.

	waiter := make(chan bool)

	go func() {
		defer close(waiter)
		for range ch {
			ws, err := unix.IoctlGetWinsize(outer, unix.TIOCGWINSZ)
			if err != nil {
				ErrLog.Printf("error getting winsz: %s", err)
				continue
			}

			err = unix.IoctlSetWinsize(inner, unix.TIOCSWINSZ, ws)
			if err != nil {
				ErrLog.Printf("error resizing pty: %s", err)
			}
		}
	}()

	signal.Notify(ch, unix.SIGWINCH)
	return func() {
		signal.Stop(ch)
		close(ch)
		<-waiter
		err = term.Restore(outer, oldState)
		if err != nil {
			ErrLog.Printf("failed to cleanup stderr: %s", err)
		}
	}
}
