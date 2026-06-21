package model

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaultsToPasstBackend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigTestFile(t, path, []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
net:
  mac_addr: 2e:33:5f:11:1b:42
`))

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got, want := cfg.Net.Backend, NetBackendPasst; got != want {
		t.Fatalf("Net.Backend = %q, want %q", got, want)
	}
	if !cfg.UsesPasstNetwork() {
		t.Fatal("UsesPasstNetwork() = false, want true")
	}
}

func TestLoadConfigMergesLegacyTapFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigTestFile(t, path, []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
net:
  backend: tap
net_mac_addr: 2e:33:5f:11:1b:42
net_if_name: vmtap-01
`))

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got, want := cfg.Net.Backend, NetBackendTap; got != want {
		t.Fatalf("Net.Backend = %q, want %q", got, want)
	}
	if got, want := cfg.Net.IfName, "vmtap-01"; got != want {
		t.Fatalf("Net.IfName = %q, want %q", got, want)
	}
	if got, want := cfg.Net.MacAddr.String(), "2e:33:5f:11:1b:42"; got != want {
		t.Fatalf("Net.MacAddr = %q, want %q", got, want)
	}
}

func TestLoadConfigRejectsTapOnlyIfNameWithoutTapBackend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigTestFile(t, path, []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
net_if_name: vmtap-01
`))

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want actionable TAP migration rejection")
	}
	if !strings.Contains(err.Error(), "net.backend: tap") {
		t.Fatalf("LoadConfig() error = %v, want net.backend: tap guidance", err)
	}
}

func TestConfigNetConfigUsesPasstVhostFields(t *testing.T) {
	cfg := &Config{Cpus: 2, Net: ManifestNetConfig{Backend: NetBackendPasst}}
	netCfg := cfg.NetConfig("/srv/vmm/nodes/node-a/run/passt.sock")
	if len(netCfg) != 1 {
		t.Fatalf("NetConfig() len = %d, want 1", len(netCfg))
	}
	if !netCfg[0].VhostUser {
		t.Fatal("VhostUser = false, want true")
	}
	if got, want := netCfg[0].VhostSocket, "/srv/vmm/nodes/node-a/run/passt.sock"; got != want {
		t.Fatalf("VhostSocket = %q, want %q", got, want)
	}
	if got, want := netCfg[0].VhostMode, "Client"; got != want {
		t.Fatalf("VhostMode = %q, want %q", got, want)
	}
	if got := netCfg[0].Tap; got != "" {
		t.Fatalf("Tap = %q, want empty for passt", got)
	}
}
