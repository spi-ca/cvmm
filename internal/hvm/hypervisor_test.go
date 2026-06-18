package hvm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/model"
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
