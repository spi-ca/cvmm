package main

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"context"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func main() {
	path := os.Args[1]

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", viper.GetString("log.prefix"), os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", viper.GetString("log.prefix"), os.Getpid()))

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

	// Expected Open from a variable.
	t, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY, 0) //nolint:gosec
	if err != nil {
		panic(err)
	}

	defer func() { _ = t.Close() }() // Best effort.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		util.InfoLog.Printf("opening console pty(%s)", path)

		// Handle pty size.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go func() {
			for range ch {
				if err := pty.InheritSize(os.Stdin, t); err != nil {
					util.ErrLog.Printf("error resizing pty: %s", err)
				}
			}
		}()

		_, _ = t.Write([]byte{'\n', '\n'})
		_ = t.Sync()
		defer func() {
			_, _ = os.Stderr.Write([]byte{'\r', '\n'})
			_ = os.Stderr.Sync()
			util.InfoLog.Printf("Bye!")
		}()
		util.InfoLog.Printf("Press ESC+( keystroke to exit this session.\r")

		<-time.After(time.Second)

		ch <- syscall.SIGWINCH                        // Initial resize.
		defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

		// Set stdin in raw mode.
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort

		go func() { _, _ = io.Copy(os.Stdout, t) }()
		go func() { util.CaptureEscapeKeySequence(os.Stdin, t); cancel() }()
		<-ctx.Done()
	} else {
		closer := make(chan struct{})
		go func() {
			defer close(closer)
			_, _ = io.Copy(t, os.Stdin)
		}()

		go func() {
			defer cancel()
			_ = t.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			_, err = io.CopyN(os.Stdout, t, 80)
			if errors.Is(err, io.EOF) {
				return
			}
			for {
				_ = t.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				writeBytes, err := io.Copy(os.Stdout, t)
				if errors.Is(err, io.EOF) {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-closer:
					if writeBytes == 0 {
						return
					}
				default:
				}
			}
		}()
		<-ctx.Done()
	}

}
