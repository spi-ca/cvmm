package main

import (
	"amuz.es/src/spi-ca/chmgr/internal/util/term_mux"
	"context"
	"fmt"
	"golang.org/x/term"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"amuz.es/src/spi-ca/chmgr/internal/util"
	"github.com/creack/pty"
	"github.com/spf13/viper"
)

func main() {
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

	cmd := exec.Command("bash") // Using bash as the child process
	ptyMaster, err := pty.Start(cmd)
	if err != nil {
		log.Fatalf("failed to start with pty: %v", err)
	}
	defer func() { _ = ptyMaster.Close() }()

	stdinfd, ptyfd := int(os.Stdin.Fd()), int(ptyMaster.Fd())

	p, err := term_mux.NewTerminalPoll()
	if err != nil {
		panic(err)
	}
	defer func() {
		if pollerErr := p.Close(); pollerErr != nil {
			util.ErrLog.Printf("failed to close TerminalPoll: %s", pollerErr)
		}
	}()

	err = p.Add(stdinfd, ptyfd)
	if err != nil {
		panic(err)
	}

	var handlers []term_mux.TerminalPollReader

	isTerminal := term.IsTerminal(stdinfd)
	if isTerminal {
		util.InfoLog.Printf("Press ESC+( keystroke to exit this session.")

		defer util.InfoLog.Printf("Bye!")
		defer func() {
			_, _ = os.Stderr.Write([]byte{'\r', '\n'})
			_ = os.Stderr.Sync()
		}()
		defer term_mux.PrepareTerminal(stdinfd, ptyfd)()

		handlers = append(handlers, term_mux.NewEscapeHandler(stdinfd))
	}

	handlers = append(handlers, term_mux.NewTerminalPollCopier(stdinfd, ptyMaster))
	handlers = append(handlers, term_mux.NewTerminalPollCopier(ptyfd, os.Stdout))

	err = p.Register(handlers...)
	if err != nil {
		panic(err)
	}

	p.Wait(ctx)
}
