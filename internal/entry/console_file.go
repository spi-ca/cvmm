package entry

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"amuz.es/src/spi-ca/chmgr/internal/util"
)

func ConsoleFile(name string, ptyId int) {
	ctx, cancel := context.WithCancel(context.Background())

	// 시그널 처리
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		select {
		case <-ctx.Done():
			return
		case sysSignal := <-exitSignal:
			util.ErrLog.Println(sysSignal.String(), " received")
			cancel()
			return
		}
	}()

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", name, os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", name, os.Getpid()))
	util.InfoLog.Print(
		"args:",
		"\n	argPtyId=", ptyId,
		"\n---",
	)

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

	stdinfd, tfd := int(os.Stdin.Fd()), int(ttyFile.Fd())

	p, err := util.NewTerminalPoll()
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			panic(fmt.Errorf("failed to close TerminalPoll: %w", closeErr))
		}
	}()

	err = p.Add(stdinfd, tfd)
	if err != nil {
		panic(err)
	}

	var handlers []util.TerminalPollReader

	isTerminal := term.IsTerminal(stdinfd)
	if isTerminal {
		util.InfoLog.Printf("opening console pty(%s)", ptyPath)
		util.InfoLog.Printf("Press ESC+( keystroke to exit this session.")

		defer util.InfoLog.Printf("Bye!")
		defer func() {
			_, _ = os.Stderr.Write([]byte{'\r', '\n'})
			_ = os.Stderr.Sync()
		}()

		terminalCloser := util.PrepareTerminal(stdinfd, tfd)
		defer terminalCloser()

		handlers = append(handlers, util.NewEscapeHandler(stdinfd))
	}

	handlers = append(handlers, util.NewTerminalPollCopier(stdinfd, ttyFile))
	handlers = append(handlers, util.NewTerminalPollCopier(tfd, os.Stdout))

	err = p.Register(handlers...)
	if err != nil {
		panic(fmt.Errorf("failed to register TerminalPollReaders: %w", err))
	}

	p.Wait(ctx)
}
