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

	isTerminal := term.IsTerminal(inputFd)
	pollCtx := ctx
	handlers := []TerminalPollReader{NewTerminalPollCopier(tfd, output)}
	pollFDs := []int{tfd}

	if isTerminal {
		pollFDs = append([]int{inputFd}, pollFDs...)
		handlers = append([]TerminalPollReader{NewTerminalPollCopier(inputFd, ttyFile)}, handlers...)

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
		var cancel context.CancelFunc
		pollCtx, cancel = context.WithCancel(ctx)
		defer cancel()

		copyDone := make(chan struct{})
		defer func() { <-copyDone }()
		go func() {
			defer close(copyDone)
			if copyErr := copyPipeInputToPTY(pollCtx, input, ttyFile); copyErr != nil && !errors.Is(copyErr, context.Canceled) {
				ErrLog.Printf("failed to copy pipe input to PTY: %v", copyErr)
			}
			cancel()
		}()

		defer func() {
			for {
				_ = ttyFile.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				writeBytes, copyErr := io.Copy(output, ttyFile)
				if errors.Is(copyErr, io.EOF) {
					return
				}
				if copyErr != nil && writeBytes == 0 {
					return
				}
				if writeBytes == 0 {
					return
				}
			}
		}()
	}

	err = p.Add(pollFDs...)
	if err != nil {
		return err
	}

	err = p.Register(handlers...)
	if err != nil {
		return fmt.Errorf("failed to register TerminalPollReaders: %w", err)
	}

	p.Wait(pollCtx)

	return nil
}

func copyPipeInputToPTY(ctx context.Context, input *os.File, ttyFile *os.File) error {
	inputFD := int(input.Fd())
	if err := unix.SetNonblock(inputFD, true); err != nil {
		return fmt.Errorf("set nonblock on stdin pipe: %w", err)
	}

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := input.Read(buf)
		if n > 0 {
			if writeErr := writePTYNonblocking(ctx, ttyFile, buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, unix.EAGAIN) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}

func writePTYNonblocking(ctx context.Context, ttyFile *os.File, buf []byte) error {
	for len(buf) > 0 {
		w, err := ttyFile.Write(buf)
		buf = buf[w:]
		if err == nil {
			continue
		}
		if errors.Is(err, unix.EAGAIN) {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(10 * time.Millisecond):
				continue
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	return nil
}
