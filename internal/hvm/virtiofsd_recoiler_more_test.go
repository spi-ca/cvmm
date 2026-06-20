package hvm

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
)

func TestVirtiofsdRecoilerFansOutMultipleSharesAndWaitsForShutdown(t *testing.T) {
	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "virtiofsd-lifecycle-helper-*")
	helperPath := buildVirtiofsdLifecycleStubBinary(t, helperDir)
	ensureVirtiofsdHelperCanStart(t, helperPath)

	readyDir := filepath.Join(tmp, "ready")
	termDir := filepath.Join(tmp, "term")
	exitDir := filepath.Join(tmp, "exit")
	if err := os.MkdirAll(readyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(termDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(exitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_VIRTIOFSD_READY_DIR", readyDir)
	t.Setenv("TEST_VIRTIOFSD_TERM_DIR", termDir)
	t.Setenv("TEST_VIRTIOFSD_EXIT_DIR", exitDir)
	t.Setenv("TEST_VIRTIOFSD_EXIT_DELAY_MS", "200")

	h := &Hypervisor{
		virtiofsdBinaryPath: helperPath,
		virtiofsdcfg: []model.VirtiofsConfig{
			{Directory: filepath.Join(tmp, "share-a"), SocketPath: filepath.Join(tmp, "share-a.sock"), ThreadPoolSize: 1},
			{Directory: filepath.Join(tmp, "share-b"), SocketPath: filepath.Join(tmp, "share-b.sock"), ThreadPoolSize: 1},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	closer := make(chan struct{})
	go h.virtiofsdRecoiler(ctx, closer)

	waitForFile(t, filepath.Join(readyDir, "share-a"), 5*time.Second)
	waitForFile(t, filepath.Join(readyDir, "share-b"), 5*time.Second)

	cancel()
	waitForFile(t, filepath.Join(termDir, "share-a"), 2*time.Second)
	waitForFile(t, filepath.Join(termDir, "share-b"), 2*time.Second)

	select {
	case <-closer:
		t.Fatal("closer closed before helpers finished exit delay")
	case <-time.After(100 * time.Millisecond):
	}

	waitForFile(t, filepath.Join(exitDir, "share-a"), 2*time.Second)
	waitForFile(t, filepath.Join(exitDir, "share-b"), 2*time.Second)
	waitForChannelClosed(t, closer, 2*time.Second)
}

func TestVirtiofsdRecoilerRestartsFailedShareUntilCanceled(t *testing.T) {
	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "virtiofsd-lifecycle-helper-*")
	helperPath := buildVirtiofsdLifecycleStubBinary(t, helperDir)
	ensureVirtiofsdHelperCanStart(t, helperPath)

	readyDir := filepath.Join(tmp, "ready")
	countDir := filepath.Join(tmp, "count")
	termDir := filepath.Join(tmp, "term")
	exitDir := filepath.Join(tmp, "exit")
	for _, dir := range []string{readyDir, countDir, termDir, exitDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("TEST_VIRTIOFSD_READY_DIR", readyDir)
	t.Setenv("TEST_VIRTIOFSD_RESTART_COUNT_DIR", countDir)
	t.Setenv("TEST_VIRTIOFSD_TERM_DIR", termDir)
	t.Setenv("TEST_VIRTIOFSD_EXIT_DIR", exitDir)
	t.Setenv("TEST_VIRTIOFSD_FAIL_UNTIL", "2")

	h := &Hypervisor{
		virtiofsdBinaryPath: helperPath,
		virtiofsdcfg: []model.VirtiofsConfig{{
			Directory:      filepath.Join(tmp, "share-restart"),
			SocketPath:     filepath.Join(tmp, "share-restart.sock"),
			ThreadPoolSize: 1,
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	closer := make(chan struct{})
	go h.virtiofsdRecoiler(ctx, closer)

	waitForCountAtLeast(t, filepath.Join(countDir, "share-restart"), 3, 5*time.Second)
	cancel()
	waitForFile(t, filepath.Join(termDir, "share-restart"), 2*time.Second)
	waitForFile(t, filepath.Join(exitDir, "share-restart"), 2*time.Second)
	waitForChannelClosed(t, closer, 2*time.Second)
}

func TestInvokeCapturesStdoutStderrAndFormatsExitErrors(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "invoke-helper.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho stdout-line\necho stderr-line >&2\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var infoBuf, errBuf bytes.Buffer
	oldInfoOut := util.InfoLog.Writer()
	oldErrOut := util.ErrLog.Writer()
	util.InfoLog.SetOutput(&infoBuf)
	util.ErrLog.SetOutput(&errBuf)
	defer util.InfoLog.SetOutput(oldInfoOut)
	defer util.ErrLog.SetOutput(oldErrOut)

	h := &Hypervisor{}
	err := h.invoke(exec.Command(scriptPath), "", nil)
	if err == nil {
		t.Fatal("invoke() error = nil, want exit error")
	}
	if !strings.Contains(infoBuf.String(), "stdout-line") {
		t.Fatalf("InfoLog = %q, want stdout capture", infoBuf.String())
	}
	if !strings.Contains(errBuf.String(), "stderr-line") {
		t.Fatalf("ErrLog = %q, want stderr capture", errBuf.String())
	}
	if !strings.Contains(err.Error(), "exit status 7") || !strings.Contains(err.Error(), "stderr-line") {
		t.Fatalf("invoke() error = %v, want exit status and stderr tail", err)
	}
}

func buildVirtiofsdLifecycleStubBinary(t *testing.T, dir string) string {
	t.Helper()

	sourcePath := filepath.Join(dir, "main.go")
	binaryPath := filepath.Join(dir, "virtiofsd-lifecycle-stub")
	source := `package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func mustMkdir(path string) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		panic(err)
	}
}

func mustWrite(path, content string) {
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
}

func markerPath(dir, id string) string {
	if dir == "" {
		return ""
	}
	mustMkdir(dir)
	return filepath.Join(dir, id)
}

func parseCount(path string) int {
	buf, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(buf)))
	if err != nil {
		panic(err)
	}
	return count
}

func writeCount(path string, count int) {
	mustWrite(path, strconv.Itoa(count))
}

func main() {
	if os.Getenv("TEST_HELPER_PROBE") == "1" {
		return
	}

	var sharedDir string
	for idx := 1; idx < len(os.Args); idx++ {
		switch os.Args[idx] {
		case "--shared-dir":
			if idx+1 < len(os.Args) {
				sharedDir = os.Args[idx+1]
				idx++
			}
		}
	}
	id := filepath.Base(sharedDir)
	if id == "." || id == string(filepath.Separator) || id == "" {
		id = "virtiofsd"
	}

	if line := os.Getenv("TEST_VIRTIOFSD_STDOUT_LINE"); line != "" {
		fmt.Println(line)
	}
	if line := os.Getenv("TEST_VIRTIOFSD_STDERR_LINE"); line != "" {
		fmt.Fprintln(os.Stderr, line)
	}

	count := 0
	if countDir := os.Getenv("TEST_VIRTIOFSD_RESTART_COUNT_DIR"); countDir != "" {
		countPath := markerPath(countDir, id)
		count = parseCount(countPath) + 1
		writeCount(countPath, count)
	}

	mustWrite(markerPath(os.Getenv("TEST_VIRTIOFSD_READY_DIR"), id), "ready")

	failUntil, _ := strconv.Atoi(os.Getenv("TEST_VIRTIOFSD_FAIL_UNTIL"))
	if count > 0 && count <= failUntil {
		os.Exit(1)
	}

	exitDelayMs, _ := strconv.Atoi(os.Getenv("TEST_VIRTIOFSD_EXIT_DELAY_MS"))
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm
	mustWrite(markerPath(os.Getenv("TEST_VIRTIOFSD_TERM_DIR"), id), "term")
	if exitDelayMs > 0 {
		time.Sleep(time.Duration(exitDelayMs) * time.Millisecond)
	}
	mustWrite(markerPath(os.Getenv("TEST_VIRTIOFSD_EXIT_DIR"), id), "exit")
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build virtiofsd lifecycle stub: %v\n%s", err, out)
	}
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	return absBinaryPath
}

func waitForCountAtLeast(t *testing.T, path string, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		buf, err := os.ReadFile(path)
		if err == nil {
			count, convErr := strconv.Atoi(strings.TrimSpace(string(buf)))
			if convErr == nil && count >= want {
				return
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("timed out waiting for %s: %v", path, err)
			}
			count, _ := strconv.Atoi(strings.TrimSpace(string(buf)))
			t.Fatalf("count in %s = %d, want >= %d", path, count, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
