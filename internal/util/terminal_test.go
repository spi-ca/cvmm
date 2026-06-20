package util

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestPrepareTerminalCopiesResizeSignalsAndRestoresState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	_, outer, _ := allocateTestPTY(t)
	defer outer.Close()
	_, inner, _ := allocateTestPTY(t)
	defer inner.Close()

	oldState, err := unix.IoctlGetTermios(int(outer.Fd()), unix.TCGETS)
	if err != nil {
		t.Skipf("get outer termios: %v", err)
	}

	setTestWinsize(t, int(outer.Fd()), 33, 111)
	setTestWinsize(t, int(inner.Fd()), 10, 20)

	restore := PrepareTerminal(int(outer.Fd()), int(inner.Fd()))
	restored := false
	defer func() {
		if !restored {
			restore()
		}
	}()

	waitForWinsize(t, int(inner.Fd()), 33, 111)

	currentState, err := unix.IoctlGetTermios(int(outer.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("get raw outer termios: %v", err)
	}
	if bytes.Equal(termiosBytes(oldState), termiosBytes(currentState)) {
		t.Fatal("PrepareTerminal() did not change terminal state")
	}

	setTestWinsize(t, int(outer.Fd()), 40, 120)
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("Kill(SIGWINCH) error = %v", err)
	}
	waitForWinsize(t, int(inner.Fd()), 40, 120)

	restore()
	restored = true
	restoredState, err := unix.IoctlGetTermios(int(outer.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("get restored outer termios: %v", err)
	}
	if !bytes.Equal(termiosBytes(oldState), termiosBytes(restoredState)) {
		t.Fatal("restore() did not restore original terminal state")
	}
}

func TestPrepareTerminalReturnsNoopWhenRawModeFails(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	var logBuf bytes.Buffer
	oldErrOut := ErrLog.Writer()
	ErrLog.SetOutput(&logBuf)
	defer ErrLog.SetOutput(oldErrOut)

	restore := PrepareTerminal(int(reader.Fd()), int(writer.Fd()))
	restore()

	if got := logBuf.String(); !strings.Contains(got, "failed to initializing pty") {
		t.Fatalf("ErrLog = %q, want raw-mode failure log", got)
	}
}

func TestPrepareTerminalLogsRestoreFailureWhenOuterClosed(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	_, outer, _ := allocateTestPTY(t)
	_, inner, _ := allocateTestPTY(t)
	defer inner.Close()

	var logBuf bytes.Buffer
	oldErrOut := ErrLog.Writer()
	ErrLog.SetOutput(&logBuf)
	defer ErrLog.SetOutput(oldErrOut)

	restore := PrepareTerminal(int(outer.Fd()), int(inner.Fd()))
	if err := outer.Close(); err != nil {
		t.Fatalf("outer.Close() error = %v", err)
	}
	restore()

	if got := logBuf.String(); !strings.Contains(got, "failed to cleanup stderr") {
		t.Fatalf("ErrLog = %q, want restore failure log", got)
	}
}

func TestOpenPtyReturnsContextualOpenError(t *testing.T) {
	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inputReader.Close()
	defer inputWriter.Close()
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()
	defer outputWriter.Close()

	missingPath := filepath.Join(t.TempDir(), "missing-pty")
	err = OpenPty(context.Background(), inputReader, outputWriter, missingPath)
	if err == nil {
		t.Fatal("OpenPty() error = nil, want open failure")
	}
	if !strings.Contains(err.Error(), "failed to open "+missingPath) {
		t.Fatalf("OpenPty() error = %v, want missing path context", err)
	}
}

func TestOpenPtyReturnsOnContextCancelWithPipeInput(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	master, slave, ptyPath := allocateTestPTY(t)
	defer master.Close()
	defer slave.Close()

	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inputReader.Close()
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- OpenPty(ctx, inputReader, outputWriter, ptyPath)
		_ = outputWriter.Close()
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	_ = inputWriter.Close()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("OpenPty() error = %v, want nil on cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OpenPty() to return after cancel")
	}
}

func TestOpenPtyCopiesPipeInputWithoutCompetingReaders(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	master, slave, ptyPath := allocateTestPTY(t)
	defer master.Close()
	defer slave.Close()

	oldState, err := term.MakeRaw(int(slave.Fd()))
	if err != nil {
		t.Skipf("MakeRaw(slave) error = %v", err)
	}
	defer func() { _ = term.Restore(int(slave.Fd()), oldState) }()

	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inputReader.Close()
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- OpenPty(ctx, inputReader, outputWriter, ptyPath)
		_ = outputWriter.Close()
	}()

	payload := bytes.Repeat([]byte("0123456789abcdef"), 32768)
	writeErrCh := make(chan error, 1)
	go func() {
		_, writeErr := inputWriter.Write(payload)
		if closeErr := inputWriter.Close(); writeErr == nil {
			writeErr = closeErr
		}
		writeErrCh <- writeErr
	}()

	time.Sleep(200 * time.Millisecond)

	if err := unix.SetNonblock(int(master.Fd()), true); err != nil {
		t.Fatalf("SetNonblock(master) error = %v", err)
	}

	received := make([]byte, len(payload))
	offset := 0
	deadline := time.Now().Add(5 * time.Second)
	for offset < len(received) && time.Now().Before(deadline) {
		n, readErr := master.Read(received[offset:])
		offset += n
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, unix.EAGAIN) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		t.Fatalf("master.Read() error = %v", readErr)
	}
	if offset != len(payload) {
		t.Fatalf("guest received %d/%d bytes from pipe input", offset, len(payload))
	}
	if !bytes.Equal(received, payload) {
		t.Fatal("guest received reordered or corrupted pipe input")
	}
	if err := <-writeErrCh; err != nil {
		t.Fatalf("inputWriter error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("OpenPty() error = %v, want nil after pipe copy", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OpenPty() to return after pipe copy")
	}
}

func TestOpenPtyCopiesGuestOutputAndReturnsOnHUP(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	master, slave, ptyPath := allocateTestPTY(t)
	defer slave.Close()

	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inputReader.Close()
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- OpenPty(ctx, inputReader, outputWriter, ptyPath)
		_ = outputWriter.Close()
	}()

	lineCh := make(chan string, 1)
	readErrCh := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(outputReader).ReadString('\n')
		if err != nil {
			readErrCh <- err
			return
		}
		lineCh <- line
	}()

	if _, err := master.Write([]byte("hello from guest\n")); err != nil {
		t.Fatalf("master.Write() error = %v", err)
	}

	select {
	case line := <-lineCh:
		if !strings.Contains(line, "hello from guest") {
			t.Fatalf("stdout line = %q, want guest output", line)
		}
	case err := <-readErrCh:
		t.Fatalf("failed to read output: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for guest output")
	}

	_ = inputWriter.Close()
	if err := master.Close(); err != nil {
		t.Fatalf("master.Close() error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("OpenPty() error = %v, want nil on HUP", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OpenPty() to return after HUP")
	}
}

func allocateTestPTY(t *testing.T) (*os.File, *os.File, string) {
	t.Helper()

	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("open /dev/ptmx: %v", err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		_ = master.Close()
		t.Skipf("unlock PTY: %v", err)
	}
	ptyID, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		_ = master.Close()
		t.Skipf("lookup PTY number: %v", err)
	}

	ptyPath := filepath.Join("/dev/pts", fmt.Sprintf("%d", ptyID))
	slave, err := os.OpenFile(ptyPath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		t.Skipf("open slave PTY: %v", err)
	}
	return master, slave, ptyPath
}

func setTestWinsize(t *testing.T, fd int, rows, cols uint16) {
	t.Helper()
	ws := &unix.Winsize{Row: rows, Col: cols}
	if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, ws); err != nil {
		t.Fatalf("IoctlSetWinsize(%d) error = %v", fd, err)
	}
}

func waitForWinsize(t *testing.T, fd int, rows, cols uint16) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
		if err == nil && ws.Row == rows && ws.Col == cols {
			return
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("timed out waiting for winsize %dx%d on fd %d: %v", rows, cols, fd, err)
			}
			t.Fatalf("timed out waiting for winsize %dx%d on fd %d, got %dx%d", rows, cols, fd, ws.Row, ws.Col)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func termiosBytes(state *unix.Termios) []byte {
	return []byte(fmt.Sprintf("%#v", *state))
}
