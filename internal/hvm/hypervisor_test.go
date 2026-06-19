package hvm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
	"golang.org/x/sys/unix"
)

func TestLoad_BuildsRuntimeConfigFromManifest(t *testing.T) {
	tmp := t.TempDir()
	imageRoot := filepath.Join(tmp, "images")
	nodeRoot := filepath.Join(tmp, "nodes")
	nodeName := "test-node"
	volatileDirectory := "run"
	manifestFilename := "config.yaml"
	kernelFilename := "vmlinuz"
	initramfsFilename := "initramfs.img"
	rootfsFilename := "root.img"
	pidFilename := "cvmm.pid"
	apiPidFilename := "cloudhypervisor.pid"
	apiSocketFilename := "api.sock"
	virtiofsSocketFilenameTemplate := "virtiofs.sock"

	nodeBasePath := filepath.Join(nodeRoot, nodeName)
	imageBasePath := filepath.Join(imageRoot, "test-image")
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)

	writeTestFile(t, filepath.Join(nodeBasePath, manifestFilename), []byte(`cpus: 2
mem: 4G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
net_mac_addr: 2e:33:5f:11:1b:42
net_if_name: vmtap-01
cmdline:
  - console=hvc0
  - quiet
disk:
  - data.img
directory:
  - configuration
`))
	writeTestFile(t, filepath.Join(imageBasePath, kernelFilename), nil)
	writeTestFile(t, filepath.Join(imageBasePath, initramfsFilename), nil)
	writeTestFile(t, filepath.Join(imageBasePath, rootfsFilename), nil)

	h, err := Load(
		nodeName,
		imageRoot, nodeRoot, volatileDirectory,
		manifestFilename,
		kernelFilename, initramfsFilename, rootfsFilename,
		pidFilename, apiPidFilename, apiSocketFilename,
		virtiofsSocketFilenameTemplate,
		"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
		false,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := h.pidPath, filepath.Join(volatileBasePath, pidFilename); got != want {
		t.Fatalf("pidPath = %q, want %q", got, want)
	}
	if got, want := h.cloudhypervisorPidPath, filepath.Join(volatileBasePath, apiPidFilename); got != want {
		t.Fatalf("cloudhypervisorPidPath = %q, want %q", got, want)
	}
	if got, want := h.cli.socketPath, filepath.Join(volatileBasePath, apiSocketFilename); got != want {
		t.Fatalf("api socket = %q, want %q", got, want)
	}

	if got, want := h.vmcfg.Payload.Kernel, filepath.Join(imageBasePath, kernelFilename); got != want {
		t.Fatalf("kernel path = %q, want %q", got, want)
	}
	if got, want := h.vmcfg.Payload.Initramfs, filepath.Join(imageBasePath, initramfsFilename); got != want {
		t.Fatalf("initramfs path = %q, want %q", got, want)
	}
	if got, want := h.vmcfg.Payload.Cmdline, "systemd.machine_id=87773d8600304db49e90e5a4314ff11b console=hvc0 console=hvc0 quiet"; got != want {
		t.Fatalf("cmdline = %q, want %q", got, want)
	}
	if got, want := h.vmcfg.Platform.SerialNumber, "87773d8600304db49e90e5a4314ff11b"; got != want {
		t.Fatalf("serial number = %q, want %q", got, want)
	}
	if got, want := h.vmcfg.Platform.OemStrings, []string{"amuzes-" + nodeName}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("oem strings = %v, want %v", got, want)
	}
	if got, want := h.vmcfg.Console.Mode, model.ConsoleModePty; got != want {
		t.Fatalf("console mode = %q, want %q", got, want)
	}

	if len(h.vmcfg.Disks) != 2 {
		t.Fatalf("disk count = %d, want 2", len(h.vmcfg.Disks))
	}
	if got, want := h.vmcfg.Disks[0].Path, filepath.Join(imageBasePath, rootfsFilename); got != want {
		t.Fatalf("rootfs path = %q, want %q", got, want)
	}
	if got, want := h.vmcfg.Disks[1].Path, filepath.Join(nodeBasePath, "data.img"); got != want {
		t.Fatalf("data disk path = %q, want %q", got, want)
	}

	if len(h.vmcfg.Fs) != 1 {
		t.Fatalf("fs count = %d, want 1", len(h.vmcfg.Fs))
	}
	if got, want := h.vmcfg.Fs[0].Tag, "configuration"; got != want {
		t.Fatalf("fs tag = %q, want %q", got, want)
	}
	if got, want := h.vmcfg.Fs[0].Socket, filepath.Join(volatileBasePath, "virtiofs_configuration.sock"); got != want {
		t.Fatalf("fs socket = %q, want %q", got, want)
	}

	if len(h.virtiofsdcfg) != 1 {
		t.Fatalf("virtiofsd config count = %d, want 1", len(h.virtiofsdcfg))
	}
	if got, want := h.virtiofsdcfg[0].Directory, filepath.Join(nodeBasePath, "configuration"); got != want {
		t.Fatalf("virtiofs directory = %q, want %q", got, want)
	}
	if got, want := h.virtiofsdcfg[0].SocketPath, filepath.Join(volatileBasePath, "virtiofs_configuration.sock"); got != want {
		t.Fatalf("virtiofs socket path = %q, want %q", got, want)
	}
}

func TestLoad_HandlesOptionalInitramfsAndAbsolutePaths(t *testing.T) {
	tmp := t.TempDir()
	imageRoot := filepath.Join(tmp, "images")
	nodeRoot := filepath.Join(tmp, "nodes")
	nodeName := "absolute-node"
	nodeBasePath := filepath.Join(nodeRoot, nodeName)
	imageBasePath := filepath.Join(imageRoot, "test-image")
	absoluteDisk := filepath.Join(tmp, "absolute-data.img")
	absoluteDirectory := filepath.Join(tmp, "absolute-share")

	writeTestFile(t, filepath.Join(nodeBasePath, "config.yaml"), []byte(`cpus: 3
mem: 2G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
disk:
  - data.img
  - `+absoluteDisk+`
directory:
  - relative-share
  - `+absoluteDirectory+`
`))
	writeTestFile(t, filepath.Join(imageBasePath, "vmlinuz"), nil)
	if err := os.MkdirAll(filepath.Join(imageBasePath, "initramfs.img"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(imageBasePath, "root.img"), nil)
	writeTestFile(t, absoluteDisk, nil)
	if err := os.MkdirAll(absoluteDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	h, err := Load(
		nodeName,
		imageRoot, nodeRoot, "run",
		"config.yaml",
		"vmlinuz", "initramfs.img", "root.img",
		"cvmm.pid", "cloudhypervisor.pid", "api.sock",
		"virtiofs.sock",
		"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
		true,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	if got := h.vmcfg.Payload.Initramfs; got != "" {
		t.Fatalf("initramfs = %q, want empty when path is a directory", got)
	}
	if got := h.vmcfg.Console.Mode; got != model.ConsoleModeTty {
		t.Fatalf("console mode = %q, want %q", got, model.ConsoleModeTty)
	}
	if len(h.vmcfg.Disks) != 3 {
		t.Fatalf("disk count = %d, want 3", len(h.vmcfg.Disks))
	}
	if got, want := h.vmcfg.Disks[1].Path, filepath.Join(nodeBasePath, "data.img"); got != want {
		t.Fatalf("relative disk path = %q, want %q", got, want)
	}
	if got := h.vmcfg.Disks[2].Path; got != absoluteDisk {
		t.Fatalf("absolute disk path = %q, want %q", got, absoluteDisk)
	}
	if len(h.virtiofsdcfg) != 2 {
		t.Fatalf("virtiofs config count = %d, want 2", len(h.virtiofsdcfg))
	}
	if got, want := h.virtiofsdcfg[0].Directory, filepath.Join(nodeBasePath, "relative-share"); got != want {
		t.Fatalf("relative directory = %q, want %q", got, want)
	}
	if got := h.virtiofsdcfg[1].Directory; got != absoluteDirectory {
		t.Fatalf("absolute directory = %q, want %q", got, absoluteDirectory)
	}
	if got, wantSuffix := h.vmcfg.Fs[1].Socket, filepath.Join(nodeBasePath, "run", "virtiofs_absolute-share.sock"); got != wantSuffix {
		t.Fatalf("absolute directory socket = %q, want %q", got, wantSuffix)
	}
	if len(h.vmcfg.Net) != 1 || len(h.vmcfg.Net[0].Mac) == 0 {
		t.Fatalf("generated MAC missing: %#v", h.vmcfg.Net)
	}
	if got := h.vmcfg.Net[0].Tap; !strings.HasPrefix(got, "vmtap-") {
		t.Fatalf("generated tap = %q, want vmtap-*", got)
	}
}

func TestLoad_ClearsMissingInitramfs(t *testing.T) {
	tmp := t.TempDir()
	imageRoot := filepath.Join(tmp, "images")
	nodeRoot := filepath.Join(tmp, "nodes")
	nodeName := "missing-initramfs"
	nodeBasePath := filepath.Join(nodeRoot, nodeName)
	imageBasePath := filepath.Join(imageRoot, "test-image")

	writeTestFile(t, filepath.Join(nodeBasePath, "config.yaml"), []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
`))
	writeTestFile(t, filepath.Join(imageBasePath, "vmlinuz"), nil)
	writeTestFile(t, filepath.Join(imageBasePath, "root.img"), nil)

	h, err := Load(
		nodeName,
		imageRoot, nodeRoot, "run",
		"config.yaml",
		"vmlinuz", "initramfs.img", "root.img",
		"cvmm.pid", "cloudhypervisor.pid", "api.sock",
		"virtiofs.sock",
		"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
		false,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.vmcfg.Payload.Initramfs; got != "" {
		t.Fatalf("initramfs = %q, want empty when file is missing", got)
	}
}

func TestLoad_ReturnsInitramfsStatErrors(t *testing.T) {
	tmp := t.TempDir()
	imageRoot := filepath.Join(tmp, "images")
	nodeRoot := filepath.Join(tmp, "nodes")
	nodeName := "bad-initramfs"
	nodeBasePath := filepath.Join(nodeRoot, nodeName)
	imageBasePath := filepath.Join(imageRoot, "test-image")

	writeTestFile(t, filepath.Join(nodeBasePath, "config.yaml"), []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
`))
	writeTestFile(t, filepath.Join(imageBasePath, "vmlinuz"), nil)
	writeTestFile(t, filepath.Join(imageBasePath, "root.img"), nil)
	writeTestFile(t, filepath.Join(imageBasePath, "blocked"), nil)

	_, err := Load(
		nodeName,
		imageRoot, nodeRoot, "run",
		"config.yaml",
		"vmlinuz", filepath.Join("blocked", "initramfs.img"), "root.img",
		"cvmm.pid", "cloudhypervisor.pid", "api.sock",
		"virtiofs.sock",
		"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
		false,
		"",
	)
	if err == nil {
		t.Fatal("Load() error = nil, want initramfs stat error")
	}
	if !strings.Contains(err.Error(), "failed to stat initramfs") {
		t.Fatalf("Load() error = %v, want initramfs context", err)
	}
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("Load() error = %v, want ENOTDIR", err)
	}
}

func TestAcquireManagerPidFileRecoversStaleMismatchedPidfile(t *testing.T) {
	h := &Hypervisor{
		name:    "managed-node",
		pidPath: filepath.Join(t.TempDir(), "cvmm.pid"),
	}
	writeTestFile(t, h.pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())))

	cleanup, err := h.acquireManagerPidFile()
	if runtime.GOOS != "linux" {
		if err == nil {
			cleanup()
			t.Fatal("acquireManagerPidFile() error = nil on non-Linux, want safe refusal")
		}
		return
	}
	if err != nil {
		t.Fatalf("acquireManagerPidFile() error = %v", err)
	}
	defer cleanup()

	pid, err := sys.ReadPidFile(h.pidPath)
	if err != nil {
		t.Fatalf("ReadPidFile() error = %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pidfile pid = %d, want %d", pid, os.Getpid())
	}
}

func TestWaitForHypervisorAvailableRespectsCancellation(t *testing.T) {
	h := &Hypervisor{cli: newClient(filepath.Join(t.TempDir(), "missing.sock"))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	err := h.waitForHypervisorAvailable(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForHypervisorAvailable() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("waitForHypervisorAvailable() took %s, want prompt cancellation", elapsed)
	}
}

func TestVirtiofsdRecoilerWaitsForHelperExitBeforeClosing(t *testing.T) {
	tmp := t.TempDir()
	readyPath := filepath.Join(tmp, "ready")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")
	helperDir := makeTestHelperDir(t, "virtiofsd-helper-*")

	t.Setenv("TEST_READY_FILE", readyPath)
	t.Setenv("TEST_TERM_FILE", termPath)
	t.Setenv("TEST_EXIT_FILE", exitPath)

	helperPath := buildVirtiofsdStubBinary(t, helperDir)
	ensureVirtiofsdHelperCanStart(t, helperPath)

	h := &Hypervisor{
		virtiofsdBinaryPath: helperPath,
		virtiofsdcfg: []model.VirtiofsConfig{{
			Directory:      tmp,
			SocketPath:     filepath.Join(tmp, "virtiofs.sock"),
			ThreadPoolSize: 1,
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closer := make(chan struct{})
	go h.virtiofsdRecoiler(ctx, closer)

	waitForFile(t, readyPath, 2*time.Second)

	select {
	case <-closer:
		t.Fatal("closer channel closed before helper exit")
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	waitForFile(t, termPath, 2*time.Second)

	select {
	case <-closer:
		t.Fatal("closer channel closed before helper finished termination")
	case <-time.After(100 * time.Millisecond):
	}

	waitForFile(t, exitPath, 3*time.Second)
	waitForChannelClosed(t, closer, 2*time.Second)
}

func TestStartUsesGracefulShutdownBeforeCancelingCloudHypervisorProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	readyPath := filepath.Join(tmp, "ready")
	bootPath := filepath.Join(tmp, "boot")
	powerPath := filepath.Join(tmp, "power")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")
	virtReadyPath := filepath.Join(tmp, "virt-ready")
	virtTermPath := filepath.Join(tmp, "virt-term")
	virtExitPath := filepath.Join(tmp, "virt-exit")
	cloudHelperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	virtiofsHelperDir := makeTestHelperDir(t, "virtiofsd-helper-*")

	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_BOOT_FILE", bootPath)
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", termPath)
	t.Setenv("TEST_CH_EXIT_FILE", exitPath)
	t.Setenv("TEST_CH_EXIT_DELAY_MS", "500")
	t.Setenv("TEST_READY_FILE", virtReadyPath)
	t.Setenv("TEST_TERM_FILE", virtTermPath)
	t.Setenv("TEST_EXIT_FILE", virtExitPath)

	helperPath := buildCloudHypervisorStubBinary(t, cloudHelperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)
	virtiofsdPath := buildVirtiofsdStubBinary(t, virtiofsHelperDir)
	ensureVirtiofsdHelperCanStart(t, virtiofsdPath)

	h := &Hypervisor{
		name:                      "graceful-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		virtiofsdBinaryPath:       virtiofsdPath,
		virtiofsdcfg: []model.VirtiofsConfig{{
			Directory:      tmp,
			SocketPath:     filepath.Join(tmp, "virtiofs.sock"),
			ThreadPoolSize: 1,
		}},
		cli: newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Start(ctx)
	}()

	waitForFile(t, virtReadyPath, 5*time.Second)
	waitForFile(t, bootPath, 5*time.Second)
	cancel()
	waitForFile(t, powerPath, 2*time.Second)
	ensureFileAbsentForDuration(t, virtTermPath, 200*time.Millisecond)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}

	waitForFile(t, exitPath, 2*time.Second)
	waitForFile(t, virtTermPath, 2*time.Second)
	waitForFile(t, virtExitPath, 3*time.Second)
	if _, err := os.Stat(termPath); err == nil {
		t.Fatal("cloud-hypervisor helper received SIGTERM before graceful power-button shutdown")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed to stat term marker: %v", err)
	}
}

func TestStartCancelsVmBootPromptlyAndStillRequestsGracefulShutdown(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	cloudHelperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	virtiofsHelperDir := makeTestHelperDir(t, "virtiofsd-helper-*")

	readyPath := filepath.Join(tmp, "ready")
	powerPath := filepath.Join(tmp, "power")
	termPath := filepath.Join(tmp, "term")
	exitPath := filepath.Join(tmp, "exit")
	virtReadyPath := filepath.Join(tmp, "virt-ready")
	virtTermPath := filepath.Join(tmp, "virt-term")
	virtExitPath := filepath.Join(tmp, "virt-exit")

	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", termPath)
	t.Setenv("TEST_CH_EXIT_FILE", exitPath)
	t.Setenv("TEST_CH_BOOT_DELAY_MS", "5000")
	t.Setenv("TEST_CH_EXIT_DELAY_MS", "500")
	t.Setenv("TEST_READY_FILE", virtReadyPath)
	t.Setenv("TEST_TERM_FILE", virtTermPath)
	t.Setenv("TEST_EXIT_FILE", virtExitPath)

	helperPath := buildCloudHypervisorStubBinary(t, cloudHelperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)
	virtiofsdPath := buildVirtiofsdStubBinary(t, virtiofsHelperDir)
	ensureVirtiofsdHelperCanStart(t, virtiofsdPath)

	h := &Hypervisor{
		name:                      "boot-cancel-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		virtiofsdBinaryPath:       virtiofsdPath,
		virtiofsdcfg: []model.VirtiofsConfig{{
			Directory:      tmp,
			SocketPath:     filepath.Join(tmp, "virtiofs.sock"),
			ThreadPoolSize: 1,
		}},
		cli: newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Start(ctx)
	}()

	waitForFile(t, readyPath, 5*time.Second)
	waitForFile(t, virtReadyPath, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	started := time.Now()
	cancel()
	waitForFile(t, powerPath, 2*time.Second)
	ensureFileAbsentForDuration(t, virtTermPath, 200*time.Millisecond)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Fatalf("Start() took %s after VmBoot cancellation, want prompt return", elapsed)
	}

	waitForFile(t, exitPath, 2*time.Second)
	waitForFile(t, virtTermPath, 2*time.Second)
	waitForFile(t, virtExitPath, 3*time.Second)
	if _, err := os.Stat(termPath); err == nil {
		t.Fatal("cloud-hypervisor helper received SIGTERM before graceful power-button shutdown during boot cancellation")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed to stat term marker: %v", err)
	}
}

func TestStartRemovesStaleCloudHypervisorPidfile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	otherReadyPath := filepath.Join(tmp, "other-ready")
	otherCmd := exec.Command(helperPath, "--api-socket", fmt.Sprintf("path=%s", filepath.Join(tmp, "other-api.sock")))
	otherCmd.Env = append(os.Environ(),
		"TEST_CH_READY_FILE="+otherReadyPath,
		"TEST_CH_BOOT_FILE="+filepath.Join(tmp, "other-boot"),
		"TEST_CH_POWER_FILE="+filepath.Join(tmp, "other-power"),
		"TEST_CH_TERM_FILE="+filepath.Join(tmp, "other-term"),
		"TEST_CH_EXIT_FILE="+filepath.Join(tmp, "other-exit"),
	)
	if err := otherCmd.Start(); err != nil {
		t.Fatalf("failed to start unrelated cloud-hypervisor helper: %v", err)
	}
	defer func() {
		_ = otherCmd.Process.Kill()
		_ = otherCmd.Wait()
	}()
	waitForFile(t, otherReadyPath, 5*time.Second)

	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

	cloudPidPath := filepath.Join(tmp, "cloudhypervisor.pid")
	writeTestFile(t, cloudPidPath, []byte(fmt.Sprintf("%d\n", otherCmd.Process.Pid)))

	h := &Hypervisor{
		name:                      "stale-pid-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    cloudPidPath,
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Start(ctx)
	}()

	waitForFile(t, filepath.Join(tmp, "boot"), 5*time.Second)
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

func TestStartRefusesMatchingCloudHypervisorPidfile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	readyPath := filepath.Join(tmp, "ready")
	cmd := exec.Command(helperPath, "--api-socket", fmt.Sprintf("path=%s", filepath.Join(tmp, "api.sock")))
	cmd.Env = append(os.Environ(),
		"TEST_CH_READY_FILE="+readyPath,
		"TEST_CH_BOOT_FILE="+filepath.Join(tmp, "boot"),
		"TEST_CH_POWER_FILE="+filepath.Join(tmp, "power"),
		"TEST_CH_TERM_FILE="+filepath.Join(tmp, "term"),
		"TEST_CH_EXIT_FILE="+filepath.Join(tmp, "exit"),
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start matching cloud-hypervisor helper: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	waitForFile(t, readyPath, 5*time.Second)

	cloudPidPath := filepath.Join(tmp, "cloudhypervisor.pid")
	writeTestFile(t, cloudPidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)))

	h := &Hypervisor{
		name:                      "active-pid-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    cloudPidPath,
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	if err := h.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want hypervisor already running")
	} else if !strings.Contains(err.Error(), "hypervisor already running") {
		t.Fatalf("Start() error = %v, want hypervisor already running", err)
	}
}

func TestLoad_ReturnsManifestErrors(t *testing.T) {
	tmp := t.TempDir()
	if _, err := Load(
		"missing-node",
		filepath.Join(tmp, "images"), filepath.Join(tmp, "nodes"), "run",
		"config.yaml",
		"vmlinuz", "initramfs.img", "root.img",
		"cvmm.pid", "cloudhypervisor.pid", "api.sock",
		"virtiofs.sock",
		"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
		false,
		"",
	); err == nil {
		t.Fatal("Load() error = nil for missing manifest, want error")
	}

	nodeBasePath := filepath.Join(tmp, "nodes", "bad-node")
	writeTestFile(t, filepath.Join(nodeBasePath, "config.yaml"), []byte("cpus: [bad\n"))
	if _, err := Load(
		"bad-node",
		filepath.Join(tmp, "images"), filepath.Join(tmp, "nodes"), "run",
		"config.yaml",
		"vmlinuz", "initramfs.img", "root.img",
		"cvmm.pid", "cloudhypervisor.pid", "api.sock",
		"virtiofs.sock",
		"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
		false,
		"",
	); err == nil {
		t.Fatal("Load() error = nil for malformed manifest, want error")
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeTestHelperDir(t *testing.T, pattern string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	helperDir, err := os.MkdirTemp(repoRoot, pattern)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(helperDir) })
	return helperDir
}

func buildVirtiofsdStubBinary(t *testing.T, dir string) string {
	t.Helper()

	sourcePath := filepath.Join(dir, "main.go")
	binaryPath := filepath.Join(dir, "virtiofsd-stub")
	source := `package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

func mustWrite(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	if os.Getenv("TEST_HELPER_PROBE") == "1" {
		return
	}

	mustWrite(os.Getenv("TEST_READY_FILE"), "ready")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
	<-ch

	mustWrite(os.Getenv("TEST_TERM_FILE"), "term")
	time.Sleep(time.Second)
	mustWrite(os.Getenv("TEST_EXIT_FILE"), "exited")
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build virtiofsd stub: %v\n%s", err, out)
	}
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	return absBinaryPath
}

func ensureVirtiofsdHelperCanStart(t *testing.T, helperPath string) {
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
	cmd.SysProcAttr.AmbientCaps = []uintptr{
		unix.CAP_CHOWN,
		unix.CAP_DAC_OVERRIDE,
		unix.CAP_FOWNER,
		unix.CAP_FSETID,
		unix.CAP_SETGID,
		unix.CAP_SETUID,
		unix.CAP_MKNOD,
		unix.CAP_SETFCAP,
		unix.CAP_DAC_READ_SEARCH,
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("ambient-capability exec unsupported in this environment: %v", err)
		}
		t.Fatalf("failed to start virtiofsd helper preflight: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("virtiofsd helper preflight wait failed: %v", err)
	}
}

func buildCloudHypervisorStubBinary(t *testing.T, dir string) string {
	t.Helper()

	sourcePath := filepath.Join(dir, "main.go")
	binaryPath := filepath.Join(dir, "cloud-hypervisor-stub")
	source := `package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func mustWrite(path, content string) {
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
}

func sleepFromEnv(key string) {
	value := os.Getenv(key)
	if value == "" {
		return
	}
	millis, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Duration(millis) * time.Millisecond)
}

func statusFromEnv(key string, defaultStatus int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultStatus
	}
	status, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return status
}

func writeResponseFromEnv(w http.ResponseWriter, statusKey, bodyKey string, defaultStatus int) {
	status := statusFromEnv(statusKey, defaultStatus)
	body := os.Getenv(bodyKey)
	w.WriteHeader(status)
	if body != "" {
		_, _ = w.Write([]byte(body))
	}
}

func main() {
	if os.Getenv("TEST_HELPER_PROBE") == "1" {
		return
	}

	var socketPath string
	for idx := 1; idx < len(os.Args); idx++ {
		if os.Args[idx] == "--api-socket" && idx+1 < len(os.Args) {
			socketPath = strings.TrimPrefix(os.Args[idx+1], "path=")
			idx++
		}
	}
	if socketPath == "" {
		panic("missing api socket path")
	}

	sleepFromEnv("TEST_CH_READY_DELAY_MS")

	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}
	defer os.Remove(socketPath)

	server := &http.Server{}
	mux := http.NewServeMux()
	server.Handler = mux

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	go func() {
		<-sigterm
		mustWrite(os.Getenv("TEST_CH_TERM_FILE"), "term")
		_ = server.Close()
		os.Exit(0)
	}()

	mux.HandleFunc("/api/v1/vmm.ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.0", "pid": 1, "features": []string{}})
	})
	mux.HandleFunc("/api/v1/vm.create", func(w http.ResponseWriter, r *http.Request) {
		sleepFromEnv("TEST_CH_CREATE_DELAY_MS")
		writeResponseFromEnv(w, "TEST_CH_CREATE_STATUS", "TEST_CH_CREATE_BODY", http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/vm.boot", func(w http.ResponseWriter, r *http.Request) {
		sleepFromEnv("TEST_CH_BOOT_DELAY_MS")
		mustWrite(os.Getenv("TEST_CH_BOOT_FILE"), "boot")
		writeResponseFromEnv(w, "TEST_CH_BOOT_STATUS", "TEST_CH_BOOT_BODY", http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/vm.power-button", func(w http.ResponseWriter, r *http.Request) {
		mustWrite(os.Getenv("TEST_CH_POWER_FILE"), "power")
		status := statusFromEnv("TEST_CH_POWER_STATUS", http.StatusNoContent)
		body := os.Getenv("TEST_CH_POWER_BODY")
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
		if status != http.StatusNoContent {
			return
		}
		go func() {
			sleepFromEnv("TEST_CH_EXIT_DELAY_MS")
			time.Sleep(50 * time.Millisecond)
			mustWrite(os.Getenv("TEST_CH_EXIT_FILE"), "exit")
			_ = server.Close()
			os.Exit(0)
		}()
	})
	mux.HandleFunc("/api/v1/vm.info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"state\":\"Running\",\"config\":{\"console\":{\"file\":\"/tmp/console\"}}}"))
	})

	mustWrite(os.Getenv("TEST_CH_READY_FILE"), "ready")
	if err := server.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build cloud-hypervisor stub: %v\n%s", err, out)
	}
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	return absBinaryPath
}

func ensureCloudHypervisorHelperCanStart(t *testing.T, helperPath string) {
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
		t.Fatalf("failed to start cloud-hypervisor helper preflight: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("cloud-hypervisor helper preflight wait failed: %v", err)
	}
}

func waitForFile(t *testing.T, path string, timeout time.Duration) {
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

func waitForChannelClosed(t *testing.T, ch <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for channel close")
	}
}

func ensureFileAbsentForDuration(t *testing.T, path string, duration time.Duration) {
	t.Helper()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s appeared before %s elapsed", path, duration)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed to stat %s: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
