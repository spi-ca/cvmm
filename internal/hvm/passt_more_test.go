package hvm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"golang.org/x/sys/unix"
)

func TestCloudHypervisorAmbientCapsDependOnNetworkBackend(t *testing.T) {
	tap := &Hypervisor{netBackend: model.NetBackendTap}
	if got, want := tap.cloudHypervisorAmbientCaps(), []uintptr{unix.CAP_NET_ADMIN}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("tap caps = %v, want %v", got, want)
	}

	passt := &Hypervisor{netBackend: model.NetBackendPasst}
	if got := passt.cloudHypervisorAmbientCaps(); len(got) != 0 {
		t.Fatalf("passt caps = %v, want none", got)
	}
}

func TestPasstCommandArgs(t *testing.T) {
	h := &Hypervisor{passtcfg: model.PasstConfig{SocketPath: "/srv/vmm/nodes/node-a/run/passt.sock", PidPath: "/srv/vmm/nodes/node-a/run/passt.pid"}}
	got := strings.Join(h.passtcfg.CommandArgs(), " ")
	for _, want := range []string{"--vhost-user", "--socket /srv/vmm/nodes/node-a/run/passt.sock", "--foreground"} {
		if !strings.Contains(got, want) {
			t.Fatalf("passt args = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "--pid") {
		t.Fatalf("passt args = %q, want no --pid", got)
	}
}

func TestStartRejectsSymlinkRuntimeDirectoryForTap(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "run")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	h := &Hypervisor{
		name:                      "tap-symlink-node",
		netBackend:                model.NetBackendTap,
		volatileBasePath:          link,
		pidPath:                   filepath.Join(link, "cvmm.pid"),
		cloudhypervisorBinaryPath: filepath.Join(tmp, "missing-cloud-hypervisor"),
		cloudhypervisorPidPath:    filepath.Join(link, "cloudhypervisor.pid"),
		cli:                       newClient(filepath.Join(link, "api.sock")),
	}
	defer h.Close()

	if err := h.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Start() error = %v, want symlink rejection", err)
	}
}

func TestValidatePasstRuntimeDirectoryRejectsUnsafeMode(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}

	h := &Hypervisor{
		volatileBasePath: runDir,
		managerUID:       uint32(os.Geteuid()),
	}
	if err := h.validatePasstRuntimeDirectory(); err == nil || !strings.Contains(err.Error(), "0700") {
		t.Fatalf("validatePasstRuntimeDirectory() error = %v, want 0700 rejection", err)
	}
}

func TestValidatePasstRuntimeDirectoryRejectsWrongOwner(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatal(err)
	}

	h := &Hypervisor{
		volatileBasePath: runDir,
		managerUID:       uint32(os.Geteuid()) + 1,
	}
	if err := h.validatePasstRuntimeDirectory(); err == nil || !strings.Contains(err.Error(), "must be owned") {
		t.Fatalf("validatePasstRuntimeDirectory() error = %v, want owner rejection", err)
	}
}

func TestValidatePasstRuntimeDirectoryRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "run")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	h := &Hypervisor{
		volatileBasePath: link,
		managerUID:       uint32(os.Geteuid()),
	}
	if err := h.validatePasstRuntimeDirectory(); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("validatePasstRuntimeDirectory() error = %v, want symlink rejection", err)
	}
}

func TestValidatePasstServiceIdentityRejectsRootOrRunAsMismatch(t *testing.T) {
	rootManager := &Hypervisor{managerUID: 0}
	if err := rootManager.validatePasstServiceIdentity(); err == nil || !strings.Contains(err.Error(), "non-root") {
		t.Fatalf("validatePasstServiceIdentity() root error = %v, want non-root rejection", err)
	}

	mismatch := &Hypervisor{
		managerUID: uint32(os.Geteuid()),
		managerGID: uint32(os.Getegid()),
		runAs:      &syscall.Credential{Uid: uint32(os.Geteuid()) + 1, Gid: uint32(os.Getegid())},
	}
	if err := mismatch.validatePasstServiceIdentity(); err == nil || !strings.Contains(err.Error(), "--runas") {
		t.Fatalf("validatePasstServiceIdentity() runas mismatch error = %v, want --runas rejection", err)
	}
}

func TestStartPasstLifecycleWritesPidAndCleansUp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process setup")
	}

	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cloudHelperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	passtHelperDir := makeTestHelperDir(t, "passt-helper-*")
	cloudPath := buildCloudHypervisorStubBinary(t, cloudHelperDir)
	passtPath := buildPasstStubBinary(t, passtHelperDir)

	createPath := filepath.Join(tmp, "create")
	powerPath := filepath.Join(tmp, "power")
	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_CREATE_FILE", createPath)
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

	passtReadyPath := filepath.Join(tmp, "passt-ready")
	passtTermPath := filepath.Join(tmp, "passt-term")
	passtExitPath := filepath.Join(tmp, "passt-exit")
	passtSelfPidPath := filepath.Join(tmp, "passt-self-pid")
	t.Setenv("TEST_PASST_READY_FILE", passtReadyPath)
	t.Setenv("TEST_PASST_TERM_FILE", passtTermPath)
	t.Setenv("TEST_PASST_EXIT_FILE", passtExitPath)
	t.Setenv("TEST_PASST_SELF_PID_FILE", passtSelfPidPath)

	h := &Hypervisor{
		name:                      "passt-node",
		netBackend:                model.NetBackendPasst,
		volatileBasePath:          runDir,
		pidPath:                   filepath.Join(runDir, "cvmm.pid"),
		cloudhypervisorBinaryPath: cloudPath,
		cloudhypervisorPidPath:    filepath.Join(runDir, "cloudhypervisor.pid"),
		passtBinaryPath:           passtPath,
		passtcfg: model.PasstConfig{
			SocketPath: filepath.Join(runDir, "passt.sock"),
			PidPath:    filepath.Join(runDir, "passt.pid"),
		},
		managerUID: uint32(os.Geteuid()),
		vmcfg: model.VmConfig{
			Payload: model.PayloadConfig{Kernel: "/kernel"},
			Net: []model.NetConfig{{
				Mac:         util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
				VhostUser:   true,
				VhostSocket: filepath.Join(runDir, "passt.sock"),
				VhostMode:   "Client",
			}},
		},
		cli: newClient(filepath.Join(runDir, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, passtReadyPath, 5*time.Second)
	waitForFile(t, createPath, 5*time.Second)
	waitForFile(t, h.passtcfg.PidPath, 5*time.Second)

	pidBytes, err := os.ReadFile(h.passtcfg.PidPath)
	if err != nil {
		t.Fatalf("ReadFile(passt pid) error = %v", err)
	}
	selfPidBytes, err := os.ReadFile(passtSelfPidPath)
	if err != nil {
		t.Fatalf("ReadFile(passt self pid) error = %v", err)
	}
	if strings.TrimSpace(string(pidBytes)) != strings.TrimSpace(string(selfPidBytes)) {
		t.Fatalf("passt pidfile = %q, want helper pid %q", strings.TrimSpace(string(pidBytes)), strings.TrimSpace(string(selfPidBytes)))
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

	waitForFile(t, powerPath, 2*time.Second)
	waitForFile(t, passtTermPath, 2*time.Second)
	waitForFile(t, passtExitPath, 2*time.Second)
	assertPathAbsent(t, h.passtcfg.PidPath)
	assertPathAbsent(t, h.passtcfg.SocketPath)
}

func TestStartWaitsForPasstSocketBeforeVmCreate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process setup")
	}

	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cloudHelperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	passtHelperDir := makeTestHelperDir(t, "passt-helper-*")
	cloudPath := buildCloudHypervisorStubBinary(t, cloudHelperDir)
	passtPath := buildPasstStubBinary(t, passtHelperDir)

	readyPath := filepath.Join(tmp, "ready")
	createPath := filepath.Join(tmp, "create")
	passtReadyPath := filepath.Join(tmp, "passt-ready")
	t.Setenv("TEST_CH_READY_FILE", readyPath)
	t.Setenv("TEST_CH_CREATE_FILE", createPath)
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))
	t.Setenv("TEST_PASST_READY_FILE", passtReadyPath)
	t.Setenv("TEST_PASST_TERM_FILE", filepath.Join(tmp, "passt-term"))
	t.Setenv("TEST_PASST_EXIT_FILE", filepath.Join(tmp, "passt-exit"))
	t.Setenv("TEST_PASST_READY_DELAY_MS", "500")

	h := &Hypervisor{
		name:                      "passt-order-node",
		netBackend:                model.NetBackendPasst,
		volatileBasePath:          runDir,
		pidPath:                   filepath.Join(runDir, "cvmm.pid"),
		cloudhypervisorBinaryPath: cloudPath,
		cloudhypervisorPidPath:    filepath.Join(runDir, "cloudhypervisor.pid"),
		passtBinaryPath:           passtPath,
		passtcfg: model.PasstConfig{
			SocketPath: filepath.Join(runDir, "passt.sock"),
			PidPath:    filepath.Join(runDir, "passt.pid"),
		},
		managerUID: uint32(os.Geteuid()),
		vmcfg: model.VmConfig{
			Payload: model.PayloadConfig{Kernel: "/kernel"},
			Net: []model.NetConfig{{
				Mac:         util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
				VhostUser:   true,
				VhostSocket: filepath.Join(runDir, "passt.sock"),
				VhostMode:   "Client",
			}},
		},
		cli: newClient(filepath.Join(runDir, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, readyPath, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(createPath); err == nil {
		cancel()
		<-errCh
		t.Fatal("vm.create happened before passt socket readiness")
	} else if !errors.Is(err, os.ErrNotExist) {
		cancel()
		<-errCh
		t.Fatalf("stat create marker: %v", err)
	}

	waitForFile(t, passtReadyPath, 5*time.Second)
	waitForFile(t, createPath, 5*time.Second)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}
}

func TestStartTreatsPasstExitAfterCreateAsFatal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process setup")
	}

	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cloudHelperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	passtHelperDir := makeTestHelperDir(t, "passt-helper-*")
	cloudPath := buildCloudHypervisorStubBinary(t, cloudHelperDir)
	passtPath := buildPasstStubBinary(t, passtHelperDir)

	createPath := filepath.Join(tmp, "create")
	powerPath := filepath.Join(tmp, "power")
	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_CREATE_FILE", createPath)
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", powerPath)
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

	t.Setenv("TEST_PASST_READY_FILE", filepath.Join(tmp, "passt-ready"))
	t.Setenv("TEST_PASST_TERM_FILE", filepath.Join(tmp, "passt-term"))
	t.Setenv("TEST_PASST_EXIT_FILE", filepath.Join(tmp, "passt-exit"))
	t.Setenv("TEST_PASST_EXIT_AFTER_READY_MS", "2000")

	h := &Hypervisor{
		name:                      "passt-fatal-node",
		netBackend:                model.NetBackendPasst,
		volatileBasePath:          runDir,
		pidPath:                   filepath.Join(runDir, "cvmm.pid"),
		cloudhypervisorBinaryPath: cloudPath,
		cloudhypervisorPidPath:    filepath.Join(runDir, "cloudhypervisor.pid"),
		passtBinaryPath:           passtPath,
		passtcfg: model.PasstConfig{
			SocketPath: filepath.Join(runDir, "passt.sock"),
			PidPath:    filepath.Join(runDir, "passt.pid"),
		},
		managerUID: uint32(os.Geteuid()),
		vmcfg: model.VmConfig{
			Payload: model.PayloadConfig{Kernel: "/kernel"},
			Net: []model.NetConfig{{
				Mac:         util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
				VhostUser:   true,
				VhostSocket: filepath.Join(runDir, "passt.sock"),
				VhostMode:   "Client",
			}},
		},
		cli: newClient(filepath.Join(runDir, "api.sock")),
	}
	defer h.Close()

	err := h.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want fatal passt exit")
	}
	if !strings.Contains(err.Error(), "passt failed after vm.create") {
		t.Fatalf("Start() error = %v, want passt fatal context", err)
	}
	waitForFile(t, createPath, 5*time.Second)
	waitForFile(t, powerPath, 2*time.Second)
	assertPathAbsent(t, h.passtcfg.PidPath)
	assertPathAbsent(t, h.passtcfg.SocketPath)
}
