package sys

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestAcquireAndReadPidFileRejectSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is platform-specific")
	}

	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	pidfile := filepath.Join(tmp, "pid")
	if err := os.WriteFile(target, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, pidfile); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadPidFile(pidfile); err == nil {
		t.Fatal("ReadPidFile(symlink) error = nil, want error")
	}
	if cleanup, err := AcquirePidFile(pidfile, os.Getpid()); err == nil {
		cleanup()
		t.Fatal("AcquirePidFile(symlink) error = nil, want error")
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep\n" {
		t.Fatalf("symlink target content = %q, want unchanged", content)
	}
}

func TestProcessIdentity(t *testing.T) {
	status, err := ProcessIdentity(-1, "node: test")
	if err != nil {
		t.Fatalf("ProcessIdentity(-1) error = %v", err)
	}
	if status != ProcessIdentityInactive {
		t.Fatalf("ProcessIdentity(-1) = %v, want %v", status, ProcessIdentityInactive)
	}

	status, err = ProcessIdentity(os.Getpid(), "node: test")
	if err != nil {
		t.Fatalf("ProcessIdentity(self) error = %v", err)
	}
	if runtime.GOOS != "linux" {
		if status != ProcessIdentityUnknown {
			t.Fatalf("ProcessIdentity(self) = %v, want %v on non-Linux", status, ProcessIdentityUnknown)
		}
		return
	}

	actualName, known, err := readProcessName(os.Getpid())
	if err != nil {
		t.Fatalf("readProcessName(self) error = %v", err)
	}
	if !known {
		t.Fatal("readProcessName(self) known = false, want true")
	}

	status, err = ProcessIdentity(os.Getpid(), actualName)
	if err != nil {
		t.Fatalf("ProcessIdentity(match) error = %v", err)
	}
	if status != ProcessIdentityMatch {
		t.Fatalf("ProcessIdentity(match) = %v, want %v", status, ProcessIdentityMatch)
	}

	status, err = ProcessIdentity(os.Getpid(), actualName+"-other")
	if err != nil {
		t.Fatalf("ProcessIdentity(mismatch) error = %v", err)
	}
	if status != ProcessIdentityMismatch {
		t.Fatalf("ProcessIdentity(mismatch) = %v, want %v", status, ProcessIdentityMismatch)
	}
}

func TestProcessIdentityWithExpectationMatchesCommandArgSubsequence(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific /proc cmdline validation")
	}

	helperPath := buildProcTestHelper(t)
	cmd := exec.Command(helperPath, "start", "managed-node")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	waitForProcessName(t, cmd.Process.Pid, filepath.Base(helperPath))
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
		CommandArgs:        []string{"start", "managed-node"},
	})
	if err != nil {
		t.Fatalf("ProcessIdentityWithExpectation(match) error = %v", err)
	}
	if status != ProcessIdentityMatch {
		t.Fatalf("ProcessIdentityWithExpectation(match) = %v, want %v", status, ProcessIdentityMatch)
	}

	status, err = ProcessIdentityWithExpectation(cmd.Process.Pid, ProcessIdentityExpectation{
		Name:               actualName,
		ExecutableBasename: filepath.Base(helperPath),
		CommandArgs:        []string{"shutdown", "managed-node"},
	})
	if err != nil {
		t.Fatalf("ProcessIdentityWithExpectation(mismatch) error = %v", err)
	}
	if status != ProcessIdentityMismatch {
		t.Fatalf("ProcessIdentityWithExpectation(mismatch) = %v, want %v", status, ProcessIdentityMismatch)
	}

	flaggedCmd := exec.Command(helperPath, "--runas", "hvm", "start", "managed-node", "--console")
	if err := flaggedCmd.Start(); err != nil {
		t.Fatalf("failed to start helper with surrounding flags: %v", err)
	}
	defer func() {
		_ = flaggedCmd.Process.Kill()
		_ = flaggedCmd.Wait()
	}()

	waitForProcessName(t, flaggedCmd.Process.Pid, actualName)
	waitForCommandArgs(t, flaggedCmd.Process.Pid, []string{"start", "managed-node"})

	status, err = ProcessIdentityWithExpectation(flaggedCmd.Process.Pid, ProcessIdentityExpectation{
		Name:               actualName,
		ExecutableBasename: filepath.Base(helperPath),
		CommandArgs:        []string{"start", "managed-node"},
	})
	if err != nil {
		t.Fatalf("ProcessIdentityWithExpectation(flagged match) error = %v", err)
	}
	if status != ProcessIdentityMatch {
		t.Fatalf("ProcessIdentityWithExpectation(flagged match) = %v, want %v", status, ProcessIdentityMatch)
	}

	splitCmd := exec.Command(helperPath, "start", "--console", "managed-node")
	if err := splitCmd.Start(); err != nil {
		t.Fatalf("failed to start helper with split command args: %v", err)
	}
	defer func() {
		_ = splitCmd.Process.Kill()
		_ = splitCmd.Wait()
	}()

	waitForProcessName(t, splitCmd.Process.Pid, actualName)
	waitForCommandArgs(t, splitCmd.Process.Pid, []string{"start", "--console", "managed-node"})

	status, err = ProcessIdentityWithExpectation(splitCmd.Process.Pid, ProcessIdentityExpectation{
		Name:               actualName,
		ExecutableBasename: filepath.Base(helperPath),
		CommandArgs:        []string{"start", "managed-node"},
	})
	if err != nil {
		t.Fatalf("ProcessIdentityWithExpectation(split match) error = %v", err)
	}
	if status != ProcessIdentityMatch {
		t.Fatalf("ProcessIdentityWithExpectation(split match) = %v, want %v", status, ProcessIdentityMatch)
	}
}

func TestWaitUntilProcessFinishedRespectsContextDeadline(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific pidfd/epoll behavior")
	}

	helperPath := buildProcTestHelper(t)
	cmd := exec.Command(helperPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	started := time.Now()
	err := WaitUntilProcessFinished(ctx, cmd.Process.Pid)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitUntilProcessFinished() error = %v, want %v", err, context.DeadlineExceeded)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("WaitUntilProcessFinished() took %s, want prompt deadline response", elapsed)
	}
}

func waitForProcessName(t *testing.T, pid int, expected string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for {
		actual, known, err := readProcessName(pid)
		if err == nil && known && actual == normalizeProcessName(expected) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for process %d name %q (last name %q, known %t, err %v)", pid, expected, actual, known, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForCommandArgs(t *testing.T, pid int, expectedSubsequence []string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for {
		cmdline, known, err := readProcessCmdline(pid)
		if err == nil && known && len(cmdline) > 0 && containsOrderedArgs(cmdline[1:], expectedSubsequence) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for process %d args %q (last cmdline %q, known %t, err %v)", pid, expectedSubsequence, cmdline, known, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func buildProcTestHelper(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	helperDir, err := os.MkdirTemp(repoRoot, "proc-helper-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(helperDir) })

	sourcePath := filepath.Join(helperDir, "main.go")
	binaryPath := filepath.Join(helperDir, "proc-helper")
	source := `package main

import "time"

func main() {
	time.Sleep(5 * time.Second)
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build proc helper: %v\n%s", err, out)
	}
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	return absBinaryPath
}

func TestNormalizeProcessName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific comm truncation")
	}

	name := "node: 12345678901234567890"
	if got, want := normalizeProcessName(name), name[:15]; got != want {
		t.Fatalf("normalizeProcessName() = %q, want %q", got, want)
	}
}
