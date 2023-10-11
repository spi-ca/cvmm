package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/creack/pty"

	"amuz.es/src/spi-ca/chmgr/internal/util"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
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
	t, err := os.OpenFile(path, os.O_RDWR|unix.O_NOCTTY, 0) //nolint:gosec
	if err != nil {
		panic(err)
	}
	defer func() { _ = t.Close() }() // Best effort.

	epfd, e := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if e != nil {
		panic(fmt.Errorf("epoll_create1: %w", e))
	}
	defer unix.Close(epfd)

	//var events [2]syscall.EpollEvent
	stdinfd := int32(os.Stdin.Fd())
	tfd := int32(t.Fd())

	err = unix.SetNonblock(int(stdinfd), true)
	if err != nil {
		panic(fmt.Errorf("SetNonblock stdinfd: %w", e))
	}

	err = unix.SetNonblock(int(tfd), true)
	if err != nil {
		panic(fmt.Errorf("SetNonblock tfd: %w", e))
	}

	err = addToEpoll(epfd, stdinfd, unix.EPOLLIN)
	if err != nil {
		log.Fatalf("failed to start with pty: %v", err)
	}

	err = addToEpoll(epfd, tfd, unix.EPOLLIN)
	if err != nil {
		log.Fatalf("failed to start with pty: %v", err)
	}

	var (
		events [32]unix.EpollEvent
		buf    [512]byte
	)

	//childstdmaster := int32(ptyMaster.Fd())
	if term.IsTerminal(int(os.Stdin.Fd())) {
		util.InfoLog.Printf("opening console pty(%s)", path)
		// Handle pty size.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, unix.SIGWINCH)
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
		oldState, err := term.MakeRaw(int(stdinfd))
		if err != nil {
			panic(err)
		}
		defer func() { _ = term.Restore(int(stdinfd), oldState) }() // Best effort

		msec := -1
		step := 0

		// Loop forever
		for {
			// Poll the file descriptor
			n, errno := unix.EpollWait(epfd, events[:], msec)
			switch errno {
			case nil:
				if n >= len(events) {
					panic(fmt.Errorf("epoll_wait: returned more than %d events", n))
				} else if n > 0 {
					msec = 0
					break
				}
				// if n <=0
				fallthrough
			case unix.EINTR:
				runtime.Gosched()
				msec = -1
				continue
			default:
				panic(errno)
			}

			// Process events
			for _, e := range events[:n] {
				if (e.Events & unix.EPOLLIN) == 0 {
					continue
				}

				switch fd := e.Fd; fd {
				case stdinfd:
					n, err := os.Stdin.Read(buf[:])

					if errors.Is(err, io.EOF) {
						return
					} else if err != nil {
						panic(err)
					}
					if n > 0 {
						if err != nil {
							return
						} else if n == 0 {
							continue
						}
						for i := 0; i < n; i++ {
							switch step {
							case 0:
								switch buf[i] {
								case 0x1b:
									step++
								}
							case 1:
								switch buf[i] {
								case 0x28:
									return
								}
								step = 0
							default:
							}
						}
						for offset := 0; offset < n; {
							written, err := t.Write(buf[offset:n])
							offset += written

							if errors.Is(err, io.EOF) {
								return
							} else if err != nil {
								panic(err)
							}
						}
					}
				case tfd:
					n, err := t.Read(buf[:])

					if errors.Is(err, io.EOF) {
						return
					} else if err != nil {
						panic(err)
					}
					if n > 0 {
						for offset := 0; offset < n; {
							written, err := os.Stdout.Write(buf[offset:n])
							offset += written

							if errors.Is(err, io.EOF) {
								return
							} else if err != nil {
								panic(err)
							}
						}
					}
				}
			}

			// Check for done signal
			select {
			case <-ctx.Done():
				return
			default:
			}
		}

	} else {

		msec := -1

		// Loop forever
		for {
			// Poll the file descriptor
			n, errno := unix.EpollWait(epfd, events[:], msec)
			switch errno {
			case nil:
				if n >= len(events) {
					panic(fmt.Errorf("epoll_wait: returned more than %d events", n))
				} else if n > 0 {
					msec = 0
					break
				}
				// if n <=0
				fallthrough
			case unix.EINTR:
				runtime.Gosched()
				msec = -1
				continue
			default:
				panic(errno)
			}

			// Process events
			for _, e := range events[:n] {
				if (e.Events & unix.EPOLLIN) == 0 {
					continue
				}

				switch fd := e.Fd; fd {
				case stdinfd:
					n, err := os.Stdin.Read(buf[:])

					if errors.Is(err, io.EOF) {
						return
					} else if err != nil {
						panic(err)
					}
					if n > 0 {
						for offset := 0; offset < n; {
							written, err := t.Write(buf[offset:n])
							offset += written

							if errors.Is(err, io.EOF) {
								return
							} else if err != nil {
								panic(err)
							}
						}
					}
				case tfd:
					n, err := t.Read(buf[:])
					if errors.Is(err, io.EOF) {
						return
					} else if err != nil {
						panic(err)
					}

					if n > 0 {
						for offset := 0; offset < n; {
							written, err := os.Stdout.Write(buf[offset:n])
							offset += written
							if errors.Is(err, io.EOF) {
								return
							} else if err != nil {
								panic(err)
							}
						}
					}
				}
			}

			// Check for done signal
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}

}

func addToEpoll(epoll int, fd int32, event int) error {
	ev := unix.EpollEvent{Events: uint32(event), Fd: fd}
	if err := unix.EpollCtl(epoll, unix.EPOLL_CTL_ADD, int(fd), &ev); err != nil {
		return fmt.Errorf("failed to add fd to epoll: %w", err)
	}
	return nil
}

func removeToEpoll(epoll int, fd int32, event int) error {
	ev := unix.EpollEvent{Events: uint32(event), Fd: fd}
	if err := unix.EpollCtl(epoll, unix.EPOLL_CTL_DEL, int(fd), &ev); err != nil {
		return fmt.Errorf("failed to remote fd to epoll: %w", err)
	}
	return nil
}
