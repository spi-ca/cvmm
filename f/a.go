package main

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"context"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"golang.org/x/term"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
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

	p, err := util.NewTerminalPoll()
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			util.ErrLog.Printf("failed to close TerminalPoll: %s", pollerErr)
		}
	}()

	err = p.Add(stdinfd, ptyfd)
	if err != nil {
		panic(err)
	}

	var handlers []util.TerminalPollReader

	isTerminal := term.IsTerminal(stdinfd)
	if isTerminal {
		util.InfoLog.Printf("Press ESC+( keystroke to exit this session.")

		defer util.InfoLog.Printf("Bye!")
		defer func() {
			_, _ = os.Stderr.Write([]byte{'\r', '\n'})
			_ = os.Stderr.Sync()
		}()
		defer util.PrepareTerminal(stdinfd, ptyfd)()

		handlers = append(handlers, util.NewEscapeHandler(stdinfd))
	} else {
		closer := make(chan struct{})
		go func() {
			defer close(closer)
			_, _ = io.Copy(ptyMaster, os.Stdin)
		}()

		defer func() {
			_ = ptyMaster.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			_, err = io.CopyN(os.Stdout, ptyMaster, 80)
			if errors.Is(err, io.EOF) {
				return
			}
			for {
				_ = ptyMaster.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				writeBytes, err := io.Copy(os.Stdout, ptyMaster)
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

	handlers = append(handlers, util.NewTerminalPollCopier(stdinfd, ptyMaster))
	handlers = append(handlers, util.NewTerminalPollCopier(ptyfd, os.Stdout))

	err = p.Register(handlers...)
	if err != nil {
		panic(err)
	}

	p.Wait(ctx)
}
