package hvm

import (
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

func TestLoadRejectsInvalidNodeNames(t *testing.T) {
	tmp := t.TempDir()
	for _, nodeName := range []string{"", ".", "..", "../escape", "nested/node", "bad name", "node..name"} {
		t.Run(nodeName, func(t *testing.T) {
			_, err := Load(
				nodeName,
				filepath.Join(tmp, "images"), filepath.Join(tmp, "nodes"), "run",
				"config.yaml",
				"vmlinuz", "initramfs.img", "root.img",
				"cvmm.pid", "cloudhypervisor.pid", "api.sock",
				"virtiofs.sock",
				"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
				false,
				"",
			)
			if err == nil {
				t.Fatal("Load() error = nil, want invalid node name rejection")
			}
			if !strings.Contains(err.Error(), "invalid node name") {
				t.Fatalf("Load() error = %v, want invalid node name context", err)
			}
		})
	}
}

func TestResolveNodeBasePathKeepsNodesInsideRoot(t *testing.T) {
	nodeRoot := filepath.Join(t.TempDir(), "nodes")
	nodeBasePath, err := resolveNodeBasePath(nodeRoot, "safe-node")
	if err != nil {
		t.Fatalf("resolveNodeBasePath() error = %v", err)
	}

	rel, err := filepath.Rel(nodeRoot, nodeBasePath)
	if err != nil {
		t.Fatalf("filepath.Rel() error = %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("resolveNodeBasePath() escaped root: nodeRoot=%q nodeBasePath=%q rel=%q", nodeRoot, nodeBasePath, rel)
	}
}

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
