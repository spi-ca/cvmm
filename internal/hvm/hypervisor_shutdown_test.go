package hvm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

func TestHypervisorManagerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HYPERVISOR_MANAGER_HELPER") != "1" {
		return
	}

	sep := -1
	for idx, arg := range os.Args {
		if arg == "--" {
			sep = idx
			break
		}
	}
	if sep < 0 || len(os.Args) <= sep+2 {
		os.Exit(2)
	}
	nodeName := os.Args[sep+2]
	_ = sys.SetProcessName(fmt.Sprintf("node: %s", nodeName))
	if readyPath := os.Getenv("TEST_MANAGER_READY_FILE"); readyPath != "" {
		_ = os.WriteFile(readyPath, []byte("ready"), 0o644)
	}

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	for {
		<-sigterm
		if termPath := os.Getenv("TEST_MANAGER_TERM_FILE"); termPath != "" {
			_ = os.WriteFile(termPath, []byte("term"), 0o644)
		}
		if os.Getenv("TEST_MANAGER_IGNORE_TERM") == "1" {
			select {}
		}
		if exitPath := os.Getenv("TEST_MANAGER_EXIT_FILE"); exitPath != "" {
			_ = os.WriteFile(exitPath, []byte("exit"), 0o644)
		}
		os.Exit(0)
	}
}

func TestShutdownRefusesMismatchedManagerPidfile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity validation")
	}

	tmp := t.TempDir()
	cmd, _, termPath, _ := startManagerHelperProcess(t, "other-node", false)
	defer stopManagerHelperProcess(cmd)

	h := &Hypervisor{name: "expected-node", pidPath: filepath.Join(tmp, "cvmm.pid")}
	writeTestFile(t, h.pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)))

	h.Shutdown(context.Background())

	ensureFileAbsentForDuration(t, termPath, 200*time.Millisecond)
	if !sys.IsPidActive(cmd.Process.Pid) {
		t.Fatal("mismatched manager process was signaled unexpectedly")
	}
}

func TestShutdownReturnsForInactivePidfile(t *testing.T) {
	h := &Hypervisor{name: "inactive-node", pidPath: filepath.Join(t.TempDir(), "cvmm.pid")}
	writeTestFile(t, h.pidPath, []byte("99999999\n"))

	started := time.Now()
	h.Shutdown(context.Background())
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("Shutdown() took %s for inactive pidfile, want prompt return", elapsed)
	}
}

func TestShutdownSignalsMatchingManager(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity validation")
	}

	tmp := t.TempDir()
	cmd, _, termPath, exitPath := startManagerHelperProcess(t, "managed-node", false)
	defer stopManagerHelperProcess(cmd)

	h := &Hypervisor{name: "managed-node", pidPath: filepath.Join(tmp, "cvmm.pid")}
	writeTestFile(t, h.pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)))

	h.Shutdown(context.Background())

	waitForFile(t, termPath, 2*time.Second)
	waitForFile(t, exitPath, 2*time.Second)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("manager helper wait error = %v", err)
	}
}

func TestShutdownKillsManagerAfterTimeout(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity validation")
	}

	oldDeadline := shutdownDeadline
	shutdownDeadline = 100 * time.Millisecond
	defer func() { shutdownDeadline = oldDeadline }()

	tmp := t.TempDir()
	cmd, _, termPath, exitPath := startManagerHelperProcess(t, "managed-node", true)
	defer stopManagerHelperProcess(cmd)

	h := &Hypervisor{name: "managed-node", pidPath: filepath.Join(tmp, "cvmm.pid")}
	writeTestFile(t, h.pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)))

	started := time.Now()
	h.Shutdown(context.Background())
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Fatalf("Shutdown() took %s, want timeout kill path", elapsed)
	}

	waitForFile(t, termPath, 2*time.Second)
	if _, err := os.Stat(exitPath); err == nil {
		t.Fatal("manager helper exited cleanly, want kill after timeout")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat exit marker: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("manager helper wait error = nil, want killed process error")
	}
}

func startManagerHelperProcess(t *testing.T, nodeName string, ignoreTerm bool) (*exec.Cmd, string, string, string) {
	t.Helper()

	tmp := t.TempDir()
	readyPath := filepath.Join(tmp, "ready")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")
	cmd := exec.Command(os.Args[0], "-test.run=TestHypervisorManagerHelperProcess", "--", "start", nodeName)
	cmd.Env = append(os.Environ(),
		"GO_WANT_HYPERVISOR_MANAGER_HELPER=1",
		"TEST_MANAGER_READY_FILE="+readyPath,
		"TEST_MANAGER_TERM_FILE="+termPath,
		"TEST_MANAGER_EXIT_FILE="+exitPath,
	)
	if ignoreTerm {
		cmd.Env = append(cmd.Env, "TEST_MANAGER_IGNORE_TERM=1")
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start manager helper: %v", err)
	}
	waitForFile(t, readyPath, 5*time.Second)
	return cmd, readyPath, termPath, exitPath
}

func stopManagerHelperProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
