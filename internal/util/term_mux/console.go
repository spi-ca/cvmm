package term_mux

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func ConsoleFile(ctx context.Context, input *os.File, output *os.File, ptyId int) {

	ptyPath := filepath.Join("/dev/pts", strconv.Itoa(ptyId))

	// Expected Open from a variable.
	ttyFile, err := os.OpenFile(ptyPath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		panic(fmt.Errorf("failed to open %s: %w", ptyPath, err))
	}
	defer func() {
		// Best effort.
		if closeErr := ttyFile.Close(); closeErr != nil {
			panic(fmt.Errorf("failed to close %s: %w", ptyPath, err))
		}
	}()

	inputFd, tfd := int(input.Fd()), int(ttyFile.Fd())

	p, err := NewTerminalPoll()
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			panic(fmt.Errorf("failed to close TerminalPoll: %w", closeErr))
		}
	}()

	err = p.Add(inputFd, tfd)
	if err != nil {
		panic(err)
	}

	var handlers []TerminalPollReader

	isTerminal := term.IsTerminal(inputFd)
	if isTerminal {
		util.InfoLog.Printf("opening console pty(%s)", ptyPath)
		util.InfoLog.Printf("Press ESC+( keystroke to exit this session.")

		defer util.InfoLog.Printf("Bye!")
		defer func() {
			_, _ = output.Write([]byte{'\r', '\n'})
			_ = output.Sync()
		}()

		terminalCloser := PrepareTerminal(inputFd, tfd)
		defer terminalCloser()

		handlers = append(handlers, NewEscapeHandler(inputFd))
	} else {
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
				default:
				}

				if writeBytes == 0 {
					break
				}
			}
		}()
	}

	handlers = append(handlers, NewTerminalPollCopier(inputFd, ttyFile))
	handlers = append(handlers, NewTerminalPollCopier(tfd, output))

	err = p.Register(handlers...)
	if err != nil {
		panic(fmt.Errorf("failed to register TerminalPollReaders: %w", err))
	}

	p.Wait(ctx)
}
