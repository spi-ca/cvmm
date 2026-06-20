package util

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func BenchmarkOpenPtyCancelLatency(b *testing.B) {
	if runtime.GOOS != "linux" {
		b.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		master, slave, ptyPath := allocateBenchmarkPTY(b)
		inputReader, inputWriter, err := os.Pipe()
		if err != nil {
			b.Fatal(err)
		}
		outputReader, outputWriter, err := os.Pipe()
		if err != nil {
			b.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)

		b.StartTimer()
		go func() {
			errCh <- OpenPty(ctx, inputReader, outputWriter, ptyPath)
			_ = outputWriter.Close()
		}()
		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				b.Fatalf("OpenPty() error = %v, want nil after cancel", err)
			}
		case <-time.After(2 * time.Second):
			b.Fatal("timed out waiting for OpenPty() to return after cancel")
		}
		b.StopTimer()

		_ = inputWriter.Close()
		_ = inputReader.Close()
		_ = outputReader.Close()
		_ = outputWriter.Close()
		_ = slave.Close()
		_ = master.Close()
		b.StartTimer()
	}
}

func allocateBenchmarkPTY(b *testing.B) (*os.File, *os.File, string) {
	b.Helper()

	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		b.Skipf("open /dev/ptmx: %v", err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		_ = master.Close()
		b.Skipf("unlock PTY: %v", err)
	}
	ptyID, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		_ = master.Close()
		b.Skipf("lookup PTY number: %v", err)
	}

	ptyPath := filepath.Join("/dev/pts", fmt.Sprintf("%d", ptyID))
	slave, err := os.OpenFile(ptyPath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		b.Skipf("open slave PTY: %v", err)
	}
	return master, slave, ptyPath
}
