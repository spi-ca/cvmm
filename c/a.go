package main

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strconv"
)

func main() {
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	fd, err := unix.PidfdOpen(pid, 0)
	if errors.Is(err, unix.ESRCH) {
		return
	} else if err != nil {
		panic(err)
	}
	defer unix.Close(fd)

	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, -1)
	if err != nil {
		panic(err)
	}

	if n != 1 {
		panic(fmt.Errorf("Ppoll: wrong number of events: got %v, expected %v", n, 1))
	}
}
