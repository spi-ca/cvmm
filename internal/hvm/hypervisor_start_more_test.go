package hvm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

func TestInvokeCleansStartedChildWhenPidfileAcquireFails(t *testing.T) {
	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)

	readyPath := filepath.Join(tmp, "ready")
	termPath := filepath.Join(tmp, "term")
	pidPath := filepath.Join(tmp, "cloudhypervisor.pid")
	targetPath := filepath.Join(tmp, "target.pid")
	if err := os.WriteFile(targetPath, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, pidPath); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_TERM_FILE", termPath)
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

	h := &Hypervisor{}
	cmd := exec.Command(helperPath, "--api-socket", "path="+filepath.Join(tmp, "api.sock"))
	err := h.invoke(cmd, pidPath, nil)
	if err == nil {
		t.Fatal("invoke() error = nil, want pidfile acquire failure")
	}
	if !strings.Contains(err.Error(), "failed to start process") {
		t.Fatalf("invoke() error = %v, want start context", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("child process state = nil, want process waited after pidfile acquire failure")
	}
}

func TestStartRejectsInvalidCloudHypervisorPidfileBeforeLaunching(t *testing.T) {
	tmp := t.TempDir()
	readyPath := filepath.Join(tmp, "ready")
	cloudPidPath := filepath.Join(tmp, "cloudhypervisor.pid")
	targetPath := filepath.Join(tmp, "target.pid")
	if err := os.WriteFile(targetPath, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, cloudPidPath); err != nil {
		t.Fatal(err)
	}

	h := &Hypervisor{
		name:                      "invalid-cloud-pid-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: filepath.Join(tmp, "missing-cloud-hypervisor"),
		cloudhypervisorPidPath:    cloudPidPath,
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()
	t.Setenv("TEST_CH_READY_FILE", readyPath)

	err := h.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want invalid pidfile error")
	}
	if !strings.Contains(err.Error(), "failed to inspect pidfile") {
		t.Fatalf("Start() error = %v, want pidfile inspection context", err)
	}
	if _, statErr := os.Stat(readyPath); statErr == nil {
		t.Fatal("hypervisor helper started despite invalid cloud-hypervisor pidfile")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("failed to stat ready marker: %v", statErr)
	}
}

func TestStartReturnsPromptlyWhenHypervisorInvokeFailsBeforeReadiness(t *testing.T) {
	tmp := t.TempDir()
	h := &Hypervisor{
		name:                      "invoke-fail-before-ready-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: filepath.Join(tmp, "missing-cloud-hypervisor"),
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	started := time.Now()
	err := h.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want invoke failure")
	}
	if !strings.Contains(err.Error(), "hypervisor failed") {
		t.Fatalf("Start() error = %v, want hypervisor failure context", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("Start() took %s, want prompt invoke failure before readiness timeout", elapsed)
	}
}

func TestStartReturnsWhenHypervisorInvokeFailsDuringImmediateCancellation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process setup")
	}

	tmp := t.TempDir()
	h := &Hypervisor{
		name:                      "invoke-fail-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: filepath.Join(tmp, "missing-cloud-hypervisor"),
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Start() error = nil, want invoke failure")
		}
		if !strings.Contains(err.Error(), "hypervisor failed") {
			t.Fatalf("Start() error = %v, want hypervisor failure context", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Start() after immediate cancellation and invoke failure")
	}
}

func TestStartRemovesInactiveCloudHypervisorPidfile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

	cloudPidPath := filepath.Join(tmp, "cloudhypervisor.pid")
	writeTestFile(t, cloudPidPath, []byte("99999999\n"))

	h := &Hypervisor{
		name:                      "inactive-pid-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    cloudPidPath,
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, filepath.Join(tmp, "boot"), 5*time.Second)
	pid, err := sys.ReadPidFile(cloudPidPath)
	if err != nil {
		t.Fatalf("ReadPidFile(cloud pid) error = %v", err)
	}
	if pid == 99999999 {
		t.Fatalf("cloud-hypervisor pidfile still contains stale pid %d", pid)
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}

	waitForFile(t, filepath.Join(tmp, "power"), 2*time.Second)
}

func TestStartStopsHypervisorWhenVmCreateFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))
	t.Setenv("TEST_CH_CREATE_STATUS", "500")
	t.Setenv("TEST_CH_CREATE_BODY", "create failed")

	h := &Hypervisor{
		name:                      "create-fail-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	err := h.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want VmCreate failure")
	}
	if !strings.Contains(err.Error(), "VmCreate") {
		t.Fatalf("Start() error = %v, want VmCreate context", err)
	}

	waitForFile(t, filepath.Join(tmp, "term"), 2*time.Second)
	if _, statErr := os.Stat(filepath.Join(tmp, "power")); statErr == nil {
		t.Fatal("power button was requested on VmCreate failure")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("failed to stat power marker: %v", statErr)
	}
	assertPathAbsent(t, h.pidPath)
	assertPathAbsent(t, h.cloudhypervisorPidPath)
}

func TestStartStopsHypervisorWhenVmBootFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))
	t.Setenv("TEST_CH_BOOT_STATUS", "500")
	t.Setenv("TEST_CH_BOOT_BODY", "boot failed")

	h := &Hypervisor{
		name:                      "boot-fail-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	err := h.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want VmBoot failure")
	}
	if !strings.Contains(err.Error(), "VmBoot") {
		t.Fatalf("Start() error = %v, want VmBoot context", err)
	}

	waitForFile(t, filepath.Join(tmp, "boot"), 2*time.Second)
	waitForFile(t, filepath.Join(tmp, "term"), 2*time.Second)
	if _, statErr := os.Stat(filepath.Join(tmp, "power")); statErr == nil {
		t.Fatal("power button was requested on VmBoot failure")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("failed to stat power marker: %v", statErr)
	}
	assertPathAbsent(t, h.pidPath)
	assertPathAbsent(t, h.cloudhypervisorPidPath)
}

func TestStartCancelsBeforeAPIReadyWithoutWaitingForShutdownDeadline(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	oldDeadline := shutdownDeadline
	shutdownDeadline = time.Second
	defer func() { shutdownDeadline = oldDeadline }()

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	readyPath := filepath.Join(tmp, "ready")
	powerPath := filepath.Join(tmp, "power")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")
	cloudPidPath := filepath.Join(tmp, "cloudhypervisor.pid")

	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", termPath)
	t.Setenv("TEST_CH_EXIT_FILE", exitPath)
	t.Setenv("TEST_CH_READY_DELAY_MS", "5000")

	h := &Hypervisor{
		name:                      "api-not-ready-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    cloudPidPath,
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, cloudPidPath, 5*time.Second)
	started := time.Now()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond {
		t.Fatalf("Start() took %s after early cancellation, want prompt return", elapsed)
	}

	waitForFile(t, termPath, 2*time.Second)
	if _, err := os.Stat(powerPath); err == nil {
		t.Fatal("power button was requested before API became ready")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat power marker: %v", err)
	}
	if _, err := os.Stat(readyPath); err == nil {
		t.Fatal("ready marker exists, want cancellation before API became ready")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat ready marker: %v", err)
	}
	if _, err := os.Stat(exitPath); err == nil {
		t.Fatal("exit marker exists, want forced termination before graceful shutdown")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat exit marker: %v", err)
	}
	assertPathAbsent(t, h.pidPath)
	assertPathAbsent(t, h.cloudhypervisorPidPath)
}

func TestStartCancelsVmCreatePromptlyWhenPowerButtonRejectsNotCreated(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	oldDeadline := shutdownDeadline
	shutdownDeadline = time.Second
	defer func() { shutdownDeadline = oldDeadline }()

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	readyPath := filepath.Join(tmp, "ready")
	powerPath := filepath.Join(tmp, "power")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")

	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", termPath)
	t.Setenv("TEST_CH_EXIT_FILE", exitPath)
	t.Setenv("TEST_CH_CREATE_DELAY_MS", "5000")
	t.Setenv("TEST_CH_POWER_STATUS", "404")

	h := &Hypervisor{
		name:                      "create-cancel-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, readyPath, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	started := time.Now()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond {
		t.Fatalf("Start() took %s after VmCreate cancellation, want prompt return", elapsed)
	}

	waitForFile(t, powerPath, 2*time.Second)
	waitForFile(t, termPath, 2*time.Second)
	if _, err := os.Stat(exitPath); err == nil {
		t.Fatal("exit marker exists, want forced termination after power-button rejection")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat exit marker: %v", err)
	}
	assertPathAbsent(t, h.pidPath)
	assertPathAbsent(t, h.cloudhypervisorPidPath)
}

func TestStartCancelsVmBootPromptlyWhenPowerButtonRejectsNotBooted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	oldDeadline := shutdownDeadline
	shutdownDeadline = time.Second
	defer func() { shutdownDeadline = oldDeadline }()

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	readyPath := filepath.Join(tmp, "ready")
	bootPath := filepath.Join(tmp, "boot")
	powerPath := filepath.Join(tmp, "power")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")

	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_BOOT_FILE", bootPath)
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", termPath)
	t.Setenv("TEST_CH_EXIT_FILE", exitPath)
	t.Setenv("TEST_CH_BOOT_DELAY_MS", "5000")
	t.Setenv("TEST_CH_POWER_STATUS", "405")

	h := &Hypervisor{
		name:                      "boot-cancel-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, readyPath, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	started := time.Now()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond {
		t.Fatalf("Start() took %s after VmBoot cancellation, want prompt return", elapsed)
	}

	waitForFile(t, powerPath, 2*time.Second)
	waitForFile(t, termPath, 2*time.Second)
	if _, err := os.Stat(bootPath); err == nil {
		t.Fatal("boot marker exists, want cancellation before guest boot completed")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat boot marker: %v", err)
	}
	if _, err := os.Stat(exitPath); err == nil {
		t.Fatal("exit marker exists, want forced termination after power-button rejection")
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat exit marker: %v", err)
	}
	assertPathAbsent(t, h.pidPath)
	assertPathAbsent(t, h.cloudhypervisorPidPath)
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists, want removed", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat %s: %v", path, err)
	}
}
