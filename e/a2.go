package main

import (
	"context"
	"fmt"
	"golang.org/x/term"
	"os"
	"os/signal"
	"syscall"

	"amuz.es/src/spi-ca/chmgr/internal/util"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
)

func main() {
	path := os.Args[1]
	ctx, cancel := context.WithCancel(context.Background())

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

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", viper.GetString("log.prefix"), os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", viper.GetString("log.prefix"), os.Getpid()))

	//

	// Expected Open from a variable.
	t, err := os.OpenFile(path, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		panic(err)
	}
	defer func() { _ = t.Close() }() // Best effort.

	stdinfd, tfd := int(os.Stdin.Fd()), int(t.Fd())

	p, err := util.NewTerminalPoll()
	if err != nil {
		panic(err)
	}
	defer func() {
		if pollerErr := p.Close(); pollerErr != nil {
			util.ErrLog.Printf("failed to close TerminalPoll: %s", pollerErr)
		}
	}()

	err = p.Add(stdinfd, tfd)
	if err != nil {
		panic(err)
	}

	var handlers []util.TerminalPollReader

	isTerminal := term.IsTerminal(stdinfd)
	if isTerminal {
		util.InfoLog.Printf("opening console pty(%s)", path)
		util.InfoLog.Printf("Press ESC+( keystroke to exit this session.")

		defer util.InfoLog.Printf("Bye!")
		defer func() {
			_, _ = os.Stderr.Write([]byte{'\r', '\n'})
			_ = os.Stderr.Sync()
		}()
		defer util.PrepareTerminal(stdinfd, tfd)()

		handlers = append(handlers, util.NewEscapeHandler(stdinfd))
	}

	handlers = append(handlers, util.NewTerminalPollCopier(stdinfd, t))
	handlers = append(handlers, util.NewTerminalPollCopier(tfd, os.Stdout))

	err = p.Register(handlers...)
	if err != nil {
		panic(err)
	}

	p.Wait(ctx)
}
