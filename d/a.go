package main

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"context"
	"fmt"
	"github.com/creack/pty"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
)

func main() {

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", viper.GetString("log.prefix"), os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", viper.GetString("log.prefix"), os.Getpid()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.Command("bash") // Using bash as the child process
	ptyMaster, err := pty.Start(cmd)
	if err != nil {
		log.Fatalf("failed to start with pty: %v", err)
	}

	epfd, e := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if e != nil {
		panic(fmt.Errorf("epoll_create1: %w", e))
	}
	defer unix.Close(epfd)

	//var events [2]syscall.EpollEvent
	stdinfd := int32(os.Stdin.Fd())
	childstdmaster := int32(ptyMaster.Fd())

	err = unix.SetNonblock(int(stdinfd), true)
	if err != nil {
		panic(fmt.Errorf("SetNonblock stdinfd: %w", e))
	}

	err = unix.SetNonblock(int(childstdmaster), true)
	if err != nil {
		panic(fmt.Errorf("SetNonblock childstdmaster: %w", e))
	}

	err = addToEpoll(epfd, stdinfd, unix.EPOLLIN)
	if err != nil {
		log.Fatalf("failed to start with pty: %v", err)
	}

	err = addToEpoll(epfd, childstdmaster, unix.EPOLLIN)
	if err != nil {
		log.Fatalf("failed to start with pty: %v", err)
	}

	var (
		events [32]unix.EpollEvent
	)
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

		//buf := [512]byte{}
		writeBuf := util.NewRingBuffer(512)
		readBuf := util.NewRingBuffer(512)

		// Process events
		for _, e := range events[:n] {
			if (e.Events & unix.EPOLLIN) == 0 {
				continue
			}

			switch fd := e.Fd; fd {
			case stdinfd:
				_, _ = io.CopyN(writeBuf, os.Stdin, 512)
				//n, _ := os.Stdin.Read(buf[:])
				//ptyMaster.Write(buf[:n])
			case childstdmaster:
				_, _ = io.CopyN(readBuf, ptyMaster, 512)
				//n, _ := ptyMaster.Read(buf[:])
				//os.Stdout.Write(buf[:n])
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

func addToEpoll(epoll int, fd int32, event int) error {
	ev := unix.EpollEvent{Events: uint32(event), Fd: fd}
	if err := unix.EpollCtl(epoll, unix.EPOLL_CTL_ADD, int(fd), &ev); err != nil {
		return fmt.Errorf("failed to add fd to epoll: %w", err)
	}
	return nil
}
