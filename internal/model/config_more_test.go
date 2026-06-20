package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/util"
	"github.com/google/uuid"
)

func TestLoadConfigOpenAndDecodeErrors(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := LoadConfig(missingPath); err == nil {
		t.Fatal("LoadConfig(missing) error = nil, want open error")
	}

	badPath := filepath.Join(t.TempDir(), "bad.yaml")
	writeConfigTestFile(t, badPath, []byte("cpus: [bad\n"))
	if _, err := LoadConfig(badPath); err == nil {
		t.Fatal("LoadConfig(malformed) error = nil, want decode error")
	}
}

func TestConfigBuildsVMAndVirtiofsConfig(t *testing.T) {
	cfg := &Config{
		Cpus:       2,
		Mem:        util.MustLoadIECSize("4G"),
		Uuid:       uuid.MustParse("87773d86-0030-4db4-9e90-e5a4314ff11b"),
		Image:      "test-image",
		NetMacAddr: util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
		NetIfName:  "vmtap-01",
		Cmdline:    []string{"quiet"},
		Disk:       []string{"data.img", "/srv/vmm/disks/archive.img"},
		Directory:  []string{"configuration", "/srv/vmm/share/absolute"},
	}

	vmcfg := cfg.VMConfig(
		"node-a",
		"/srv/vmm/images/test-image/vmlinuz",
		"/srv/vmm/images/test-image/initramfs.img",
		"/srv/vmm/images/test-image/root.img",
		"/srv/vmm/nodes/node-a",
		"/srv/vmm/nodes/node-a/run/virtiofs.sock",
		true,
	)
	if got, want := vmcfg.Payload.Cmdline, "systemd.machine_id=87773d8600304db49e90e5a4314ff11b console=hvc0 quiet"; got != want {
		t.Fatalf("KernelCommandline() = %q, want %q", got, want)
	}
	if got := vmcfg.Console.Mode; got != ConsoleModeTty {
		t.Fatalf("ConsoleConfig(std=true) mode = %q, want %q", got, ConsoleModeTty)
	}
	if got, want := vmcfg.Disks[1].Path, filepath.Join("/srv/vmm/nodes/node-a", "data.img"); got != want {
		t.Fatalf("relative disk path = %q, want %q", got, want)
	}
	if got := vmcfg.Disks[2].Path; got != "/srv/vmm/disks/archive.img" {
		t.Fatalf("absolute disk path = %q, want absolute path", got)
	}
	if got := vmcfg.Fs[0].Socket; !strings.HasSuffix(got, "virtiofs_configuration.sock") {
		t.Fatalf("relative fs socket = %q, want suffixed socket", got)
	}
	if got := vmcfg.Fs[1].Tag; got != "absolute" {
		t.Fatalf("absolute fs tag = %q, want basename", got)
	}

	virtiofsCfg := cfg.VirtiofsConfig("/srv/vmm/nodes/node-a", "/srv/vmm/nodes/node-a/run/virtiofs.sock", "/srv/vmm/nodes/node-a/run/virtiofs.pid", "kvm")
	if got, want := virtiofsCfg[0].Directory, filepath.Join("/srv/vmm/nodes/node-a", "configuration"); got != want {
		t.Fatalf("relative virtiofs directory = %q, want %q", got, want)
	}
	if got, want := virtiofsCfg[0].PidPath, "/srv/vmm/nodes/node-a/run/virtiofs_configuration.pid"; got != want {
		t.Fatalf("relative virtiofs pid path = %q, want %q", got, want)
	}
	if got := virtiofsCfg[0].SocketGroup; got != "kvm" {
		t.Fatalf("SocketGroup = %q, want kvm", got)
	}
	if got := virtiofsCfg[1].Directory; got != "/srv/vmm/share/absolute" {
		t.Fatalf("absolute virtiofs directory = %q, want absolute path", got)
	}
}

func writeConfigTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
