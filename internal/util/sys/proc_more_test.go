package sys

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestReadAndAcquirePidFileRejectNamedPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipes differ on Windows")
	}

	tmp := t.TempDir()
	pidfile := filepath.Join(tmp, "pid.fifo")
	if err := syscall.Mkfifo(pidfile, 0o600); err != nil {
		t.Fatalf("Mkfifo() error = %v", err)
	}

	holder, err := os.OpenFile(pidfile, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", pidfile, err)
	}
	defer holder.Close()

	if _, err := ReadPidFile(pidfile); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("ReadPidFile(named pipe) error = %v, want non-regular file error", err)
	}
	if cleanup, err := AcquirePidFile(pidfile, os.Getpid()); err == nil {
		cleanup()
		t.Fatal("AcquirePidFile(named pipe) error = nil, want error")
	} else if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("AcquirePidFile(named pipe) error = %v, want non-regular file error", err)
	}
}

func TestAcquirePidFileRejectsActivePidfile(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "cvmm.pid")
	writeProcTestFile(t, pidfile, []byte(fmt.Sprintf("%d\n", os.Getpid())))

	if cleanup, err := AcquirePidFile(pidfile, os.Getpid()); err == nil {
		cleanup()
		t.Fatal("AcquirePidFile(active pidfile) error = nil, want already running")
	} else if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("AcquirePidFile(active pidfile) error = %v, want already running", err)
	}
}

func TestAcquirePidFileCleanupOnlyRemovesOwnedFile(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "cvmm.pid")
	cleanup, err := AcquirePidFile(pidfile, os.Getpid())
	if err != nil {
		t.Fatalf("AcquirePidFile() error = %v", err)
	}

	replacedPath := filepath.Join(filepath.Dir(pidfile), "cvmm-original.pid")
	if err := os.Rename(pidfile, replacedPath); err != nil {
		cleanup()
		t.Fatalf("Rename() error = %v", err)
	}
	writeProcTestFile(t, pidfile, []byte("replacement\n"))

	cleanup()

	if content, err := os.ReadFile(pidfile); err != nil {
		t.Fatalf("ReadFile(replacement) error = %v", err)
	} else if string(content) != "replacement\n" {
		t.Fatalf("replacement pidfile content = %q, want preserved", content)
	}
}

func TestReadPidFileRejectsInvalidContentAndLongFirstLine(t *testing.T) {
	tmp := t.TempDir()
	invalidPath := filepath.Join(tmp, "invalid.pid")
	writeProcTestFile(t, invalidPath, []byte("not-a-pid\n"))

	if _, err := ReadPidFile(invalidPath); err == nil {
		t.Fatal("ReadPidFile(invalid content) error = nil, want error")
	}

	longPath := filepath.Join(tmp, "long.pid")
	writeProcTestFile(t, longPath, []byte(strings.Repeat("1", 5000)+"\n"))
	if _, err := ReadPidFile(longPath); err == nil {
		t.Fatal("ReadPidFile(long line) error = nil, want error")
	} else if !strings.Contains(err.Error(), "first line is too long") {
		t.Fatalf("ReadPidFile(long line) error = %v, want long-line error", err)
	}
}

func TestProcessIdentityWithExpectationRejectsWrongNodeAndSocket(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific /proc identity validation")
	}

	helperPath := buildProcTestHelper(t)
	cmd := exec.Command(helperPath, "--api-socket", "path=/tmp/socket-a", "start", "managed-node")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	waitForCommandArgs(t, cmd.Process.Pid, []string{"start", "managed-node"})
	actualName, known, err := readProcessName(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("readProcessName(helper) error = %v", err)
	}
	if !known {
		t.Fatal("readProcessName(helper) known = false, want true")
	}

	status, err := ProcessIdentityWithExpectation(cmd.Process.Pid, ProcessIdentityExpectation{
		Name:               actualName,
		ExecutableBasename: filepath.Base(helperPath),
		CommandArgs:        []string{"start", "other-node"},
	})
	if err != nil {
		t.Fatalf("ProcessIdentityWithExpectation(wrong node) error = %v", err)
	}
	if status != ProcessIdentityMismatch {
		t.Fatalf("ProcessIdentityWithExpectation(wrong node) = %v, want %v", status, ProcessIdentityMismatch)
	}

	status, err = ProcessIdentityWithExpectation(cmd.Process.Pid, ProcessIdentityExpectation{
		Name:               actualName,
		ExecutableBasename: filepath.Base(helperPath),
		CommandArgs:        []string{"--api-socket", "path=/tmp/socket-b"},
	})
	if err != nil {
		t.Fatalf("ProcessIdentityWithExpectation(wrong socket) error = %v", err)
	}
	if status != ProcessIdentityMismatch {
		t.Fatalf("ProcessIdentityWithExpectation(wrong socket) = %v, want %v", status, ProcessIdentityMismatch)
	}
}

func writeProcTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
