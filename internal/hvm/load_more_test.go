package hvm

import (
	"os/user"
	"path/filepath"
	"reflect"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

func TestLoadRunAsPropagatesCredentialAndSocketGroup(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Skipf("current user lookup unavailable: %v", err)
	}

	tmp := t.TempDir()
	imageRoot := filepath.Join(tmp, "images")
	nodeRoot := filepath.Join(tmp, "nodes")
	nodeName := "runas-node"
	nodeBasePath := filepath.Join(nodeRoot, nodeName)
	imageBasePath := filepath.Join(imageRoot, "test-image")

	writeTestFile(t, filepath.Join(nodeBasePath, "config.yaml"), []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
directory:
  - configuration
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
		currentUser.Username,
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	defer h.Close()

	expectedCredential, err := sys.LookupCredentials(currentUser.Username)
	if err != nil {
		t.Fatalf("LookupCredentials() error = %v", err)
	}
	if !reflect.DeepEqual(h.runAs, expectedCredential) {
		t.Fatalf("runAs = %#v, want %#v", h.runAs, expectedCredential)
	}

	expectedGroup, err := sys.LookupGroupName(expectedCredential.Gid)
	if err != nil {
		t.Fatalf("LookupGroupName() error = %v", err)
	}
	if len(h.virtiofsdcfg) != 1 {
		t.Fatalf("virtiofsd config count = %d, want 1", len(h.virtiofsdcfg))
	}
	if got := h.virtiofsdcfg[0].SocketGroup; got != expectedGroup {
		t.Fatalf("SocketGroup = %q, want %q", got, expectedGroup)
	}
}
