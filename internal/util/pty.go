package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// OpenPty connects stdin/stdout to a guest PTY until either side closes or the context ends.
func OpenPty(ctx context.Context, input *os.File, output *os.File, ptyPath string) error {
	// Open the PTY path reported by cloud-hypervisor rather than constructing a path locally.
	ttyFile, err := os.OpenFile(ptyPath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", ptyPath, err)
	}
	defer func() {
		// Closing the PTY during cleanup is best-effort because the command is already ending.
		_ = ttyFile.Close()
	}()

	inputFd, tfd := int(input.Fd()), int(ttyFile.Fd())

	p, err := NewTerminalPoll()
	if err != nil {
		return err
	}
	defer func() {
		_ = p.Close()
	}()

	err = p.Add(inputFd, tfd)
	if err != nil {
		return err
	}

	var handlers []TerminalPollReader

	isTerminal := term.IsTerminal(inputFd)
	if isTerminal {
		InfoLog.Printf("opening console pty(%s)", ptyPath)
		InfoLog.Printf("Press ESC+( keystroke to exit this session.")

		defer InfoLog.Printf("Bye!")
		defer func() {
			_, _ = output.Write([]byte{'\r', '\n'})
			_ = output.Sync()
		}()

		terminalCloser := PrepareTerminal(inputFd, tfd)
		defer terminalCloser()

		handlers = append(handlers, NewEscapeHandler(inputFd))
	} else {
		closer := make(chan struct{})
		go func() {
			defer close(closer)
			_, _ = io.Copy(ttyFile, os.Stdin)
		}()

		defer func() {
			_ = ttyFile.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			_, err = io.CopyN(output, ttyFile, 80)
			if errors.Is(err, io.EOF) {
				return
			}
			for {
				_ = ttyFile.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				writeBytes, err := io.Copy(output, ttyFile)
				if err != nil {
					break
				}
				select {
				case <-ctx.Done():
					break
				case <-closer:
					if writeBytes == 0 {
						break
					}
				default:
				}
			}
		}()
	}

	handlers = append(handlers, NewTerminalPollCopier(inputFd, ttyFile))
	handlers = append(handlers, NewTerminalPollCopier(tfd, output))

	err = p.Register(handlers...)
	if err != nil {
		return fmt.Errorf("failed to register TerminalPollReaders: %w", err)
	}

	p.Wait(ctx)

	return nil
}
