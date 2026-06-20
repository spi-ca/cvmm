package entry

import (
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
)

// ConsoleFile handles the cvmm console-file command entrypoint for a numeric PTY id.
func ConsoleFile(name string, ptyId int) {
	ctx, cancel := context.WithCancel(context.Background())

	// ConsoleFile stops PTY forwarding when the command receives a termination signal.
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(exitSignal)
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

	ptyPath := consolePtyPath(ptyId)
	if err := util.ValidateDirectConsolePTYPath(ptyPath); err != nil {
		panic(err)
	}

	err := util.OpenPty(ctx, os.Stdin, os.Stdout, ptyPath)
	if err != nil {
		panic(err)
	}
}

func consolePtyPath(ptyId int) string {
	return filepath.Join("/dev/pts", strconv.Itoa(ptyId))
}
