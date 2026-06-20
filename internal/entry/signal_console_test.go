package entry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
	"golang.org/x/sys/unix"
)

func TestEntryHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ENTRY_HELPER") != "1" {
		return
	}

	setupClientViperFromEnv()
	switch os.Getenv("TEST_ENTRY_ACTION") {
	case "start":
		Start("cvmm", os.Getenv("TEST_CLIENT_NODE_NAME"))
	case "shutdown":
		Shutdown("cvmm", os.Getenv("TEST_CLIENT_NODE_NAME"))
	case "console":
		Console("cvmm", os.Getenv("TEST_CLIENT_NODE_NAME"))
	case "console-file":
		ptyID, err := strconv.Atoi(os.Getenv("TEST_ENTRY_PTY_ID"))
		if err != nil {
			panic(err)
		}
		ConsoleFile("cvmm", ptyID)
	default:
		os.Exit(2)
	}
}

func TestEntryManagerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ENTRY_MANAGER_HELPER") != "1" {
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

func TestStartSignalCancelsBeforeReadiness(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal semantics required")
	}

	rt := setupClientRuntime(t)
	childReadyPath := filepath.Join(t.TempDir(), "child-ready")
	childTermPath := filepath.Join(t.TempDir(), "child-term")
	cloudPath := buildEntryWaitBinary(t)
	ensureEntryWaitBinaryCanStart(t, cloudPath)

	cmd := exec.Command(os.Args[0], "-test.run=TestEntryHelperProcess")
	cmd.Env = append(entryHelperEnv(rt),
		"TEST_ENTRY_ACTION=start",
		"TEST_CLIENT_CLOUD_PATH="+cloudPath,
		"TEST_ENTRY_WAIT_READY_FILE="+childReadyPath,
		"TEST_ENTRY_WAIT_TERM_FILE="+childTermPath,
	)
	var stdout, stderr safeBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("helper start error = %v", err)
	}

	waitForPath(t, childReadyPath, 5*time.Second)
	assertHelperSignalExit(t, cmd, syscall.SIGTERM, 5*time.Second)
	waitForPath(t, childTermPath, 2*time.Second)
	assertSignalLog(t, stdout.String()+stderr.String(), syscall.SIGTERM)
}

func TestShutdownSignalCancelsWaitAndKillsManager(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux process identity validation required")
	}

	rt := setupClientRuntime(t)
	managerCmd, termPath := startEntryManagerHelperProcess(t, rt.nodeName, true)
	defer stopEntryManagerHelperProcess(managerCmd)

	pidPath := filepath.Join(rt.nodeRoot, rt.nodeName, "run", "cvmm.pid")
	writeClientTestFile(t, pidPath, []byte(fmt.Sprintf("%d\n", managerCmd.Process.Pid)))

	cmd := exec.Command(os.Args[0], "-test.run=TestEntryHelperProcess")
	cmd.Env = append(entryHelperEnv(rt), "TEST_ENTRY_ACTION=shutdown")
	var stdout, stderr safeBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("helper start error = %v", err)
	}

	waitForPath(t, termPath, 5*time.Second)
	assertHelperSignalExit(t, cmd, syscall.SIGINT, 5*time.Second)
	assertSignalLog(t, stdout.String()+stderr.String(), syscall.SIGINT)

	waitDone := make(chan error, 1)
	go func() { waitDone <- managerCmd.Wait() }()
	select {
	case err := <-waitDone:
		if err == nil {
			t.Fatal("manager wait error = nil, want forced kill after helper cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for manager helper to exit")
	}
}

func TestConsoleSignalCancelsAttachedPTYSession(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux PTY allocation helper required")
	}

	rt := setupClientRuntime(t)
	master, _, ptyPath := allocateEntryTestPTY(t)
	defer master.Close()

	_, _ = withClientUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&model.VmInfo{
			State:  model.NodeStatusRunning,
			Config: model.VmConfig{Console: &model.ConsoleConfig{File: ptyPath, Mode: model.ConsoleModePty}},
		})
	})

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinWriter.Close()

	cmd := exec.Command(os.Args[0], "-test.run=TestEntryHelperProcess")
	cmd.Env = append(entryHelperEnv(rt), "TEST_ENTRY_ACTION=console")
	cmd.Stdin = stdinReader
	var stdout, stderr safeBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("helper start error = %v", err)
	}
	_ = stdinReader.Close()

	if _, err := master.Write([]byte("hello console\n")); err != nil {
		t.Fatalf("master.Write() error = %v", err)
	}
	waitForSubstring(t, &stdout, "hello console", 2*time.Second)

	assertHelperSignalExit(t, cmd, syscall.SIGHUP, 5*time.Second)
	assertSignalLog(t, stdout.String()+stderr.String(), syscall.SIGHUP)
}

func TestConsoleFileSignalCancelsAttachedPTYSession(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux PTY allocation helper required")
	}

	master, _, ptyPath := allocateEntryTestPTY(t)
	defer master.Close()
	ptyID, err := strconv.Atoi(filepath.Base(ptyPath))
	if err != nil {
		t.Fatalf("invalid PTY path %q: %v", ptyPath, err)
	}

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinWriter.Close()

	cmd := exec.Command(os.Args[0], "-test.run=TestEntryHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_ENTRY_HELPER=1",
		"TEST_ENTRY_ACTION=console-file",
		"TEST_ENTRY_PTY_ID="+strconv.Itoa(ptyID),
	)
	cmd.Stdin = stdinReader
	var stdout, stderr safeBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("helper start error = %v", err)
	}
	_ = stdinReader.Close()

	if _, err := master.Write([]byte("hello console-file\n")); err != nil {
		t.Fatalf("master.Write() error = %v", err)
	}
	waitForSubstring(t, &stdout, "hello console-file", 2*time.Second)

	assertHelperSignalExit(t, cmd, syscall.SIGQUIT, 5*time.Second)
	assertSignalLog(t, stdout.String()+stderr.String(), syscall.SIGQUIT)
}

func TestConsoleFilePanicsForMissingPTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX PTY paths required")
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestEntryHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_ENTRY_HELPER=1",
		"TEST_ENTRY_ACTION=console-file",
		"TEST_ENTRY_PTY_ID=999999",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("helper run error = nil, want panic for missing PTY")
	}
	if !strings.Contains(string(output), "invalid console PTY path") {
		t.Fatalf("output = %s, want console PTY validation panic", output)
	}
}

func entryHelperEnv(rt *clientRuntime) []string {
	return append(os.Environ(),
		"GO_WANT_ENTRY_HELPER=1",
		"TEST_CLIENT_NODE_NAME="+rt.nodeName,
		"TEST_CLIENT_IMAGE_ROOT="+rt.imageRoot,
		"TEST_CLIENT_NODE_ROOT="+rt.nodeRoot,
		"TEST_CLIENT_VOLATILE_DIR=run",
		"TEST_CLIENT_MANIFEST_FILENAME=config.yaml",
		"TEST_CLIENT_KERNEL_FILENAME=vmlinuz",
		"TEST_CLIENT_INITRAMFS_FILENAME=initramfs.img",
		"TEST_CLIENT_ROOTFS_FILENAME=root.img",
		"TEST_CLIENT_PID_FILENAME=cvmm.pid",
		"TEST_CLIENT_CLOUD_PID_FILENAME=cloudhypervisor.pid",
		"TEST_CLIENT_API_FILENAME=api.sock",
		"TEST_CLIENT_VIRTIOFS_TEMPLATE=virtiofs.sock",
		"TEST_CLIENT_CLOUD_PATH=sh",
		"TEST_CLIENT_VIRTIOFSD_PATH=sh",
		"TEST_CLIENT_RUNAS=",
	)
}

func buildEntryWaitBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	binaryPath := filepath.Join(dir, "cloud-hypervisor-wait")
	source := `package main

import (
	"os"
	"os/signal"
	"syscall"
)

func mustWrite(path string) {
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte("ready"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	if os.Getenv("TEST_HELPER_PROBE") == "1" {
		return
	}
	mustWrite(os.Getenv("TEST_ENTRY_WAIT_READY_FILE"))
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
	<-ch
	mustWrite(os.Getenv("TEST_ENTRY_WAIT_TERM_FILE"))
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build entry wait helper: %v\n%s", err, out)
	}
	return binaryPath
}

func ensureEntryWaitBinaryCanStart(t *testing.T, helperPath string) {
	t.Helper()
	cmd := exec.Command(helperPath)
	cmd.Env = append(os.Environ(), "TEST_HELPER_PROBE=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	if err := sys.ApplySysProAttrPGid(cmd.SysProcAttr); err != nil {
		t.Fatal(err)
	}
	if err := sys.ApplySysProAttrPdeathsig(cmd.SysProcAttr, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	cmd.SysProcAttr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("ambient-capability exec unsupported in this environment: %v", err)
		}
		t.Fatalf("failed to start entry wait helper preflight: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("entry wait helper preflight wait failed: %v", err)
	}
}

func startEntryManagerHelperProcess(t *testing.T, nodeName string, ignoreTerm bool) (*exec.Cmd, string) {
	t.Helper()
	tmp := t.TempDir()
	readyPath := filepath.Join(tmp, "ready")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")
	cmd := exec.Command(os.Args[0], "-test.run=TestEntryManagerHelperProcess", "--", "start", nodeName)
	cmd.Env = append(os.Environ(),
		"GO_WANT_ENTRY_MANAGER_HELPER=1",
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
	waitForPath(t, readyPath, 5*time.Second)
	return cmd, termPath
}

func stopEntryManagerHelperProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func allocateEntryTestPTY(t *testing.T) (*os.File, *os.File, string) {
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
	ptyPath := filepath.Join("/dev/pts", strconv.Itoa(ptyID))
	slave, err := os.OpenFile(ptyPath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		t.Skipf("open slave PTY: %v", err)
	}
	return master, slave, ptyPath
}

func waitForPath(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForSubstring(t *testing.T, buf interface{ String() string }, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if strings.Contains(buf.String(), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q in %q", want, buf.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertHelperSignalExit(t *testing.T, cmd *exec.Cmd, sig syscall.Signal, timeout time.Duration) {
	t.Helper()
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatalf("Signal(%s) error = %v", sig, err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	select {
	case err := <-waitCh:
		if err != nil {
			t.Fatalf("helper wait error = %v", err)
		}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		t.Fatalf("timed out waiting for helper to exit after %s", sig)
	}
}

func assertSignalLog(t *testing.T, output string, sig syscall.Signal) {
	t.Helper()
	if !strings.Contains(output, sig.String()) || !strings.Contains(output, "received") {
		t.Fatalf("output = %s, want signal receipt log for %s", output, sig)
	}
}
